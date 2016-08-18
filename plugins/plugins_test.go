package plugins_test

import (
  "reflect"
  "os"
  "io"
  "path/filepath"
  "fmt"
  "net/http"
  "net/http/httptest"

  "github.com/bouk/monkey"
  . "github.com/onsi/ginkgo"
  . "github.com/onsi/gomega"

  "github.com/markelog/eclectica/variables"
  "github.com/markelog/eclectica/plugins/nodejs"
  ."github.com/markelog/eclectica/plugins"
)

var _ = Describe("plugins", func() {
  var (
    name string
    path string
    version string
    archivePath string
    destFolder string
    versionsFolder string
    url string
    filename string
    info map[string]string
    plugin *Plugin
  )

  Describe("Install", func() {
    It("returns error if version was not defined", func() {
      plugin := New("node")
      Expect(plugin.Install()).Should(MatchError("Version was not defined"))
    })
  })

  Describe("Activate", func() {
    It("should call Extract then Install methods", func() {
      result := ""

      plugin := New("node", "5.0.0")
      ptype := reflect.TypeOf(plugin)

      var guardExtract *monkey.PatchGuard
      guardExtract = monkey.PatchInstanceMethod(ptype, "Extract", func(*Plugin) error {
        result += "Extract"

        guardExtract.Unpatch()

        return nil
      })

      var guardInstall *monkey.PatchGuard
      guardInstall = monkey.PatchInstanceMethod(ptype, "Install", func(*Plugin) error {
        result += "Install"

        guardInstall.Unpatch()

        return nil
      })

      plugin.Activate()

      Expect(result).To(Equal("ExtractInstall"))
    })
  })

  Describe("Extract", func() {
    BeforeEach(func() {
      path, _ = filepath.Abs("../testdata/plugins")
      versionsFolder, _ = filepath.Abs("../testdata/plugins/versions")
      name = "node"
      version = "1.0.0"
      filename = "node-arch"
      destFolder, _ = filepath.Abs("../testdata/plugins/versions/" + name + "/" + version)
      archivePath = path + "/" + filename + ".tar.gz"

      info = map[string]string{
        "name": name,
        "version": version,
        "archive-path": archivePath,
        "destination-folder": destFolder,
        "filename": filename,
      }

      monkey.Patch(variables.Home, func() string {
        return versionsFolder
      })

      var d *Plugin
      ptype := reflect.TypeOf(d)

      var guard *monkey.PatchGuard
      guard = monkey.PatchInstanceMethod(ptype, "Info",
        func(plugin *Plugin) (map[string]string, error) {
          guard.Unpatch()
          return info, nil
        })

      plugin = New("node", "5.0.0")
    })

    AfterEach(func() {
      monkey.Unpatch(variables.Home)
      os.RemoveAll(versionsFolder + "/" + name)
    })

    It("returns error if version was not defined", func() {
      plugin := New("node")
      Expect(plugin.Extract()).Should(MatchError("Version was not defined"))
    })

    It("should extract langauge", func() {
      plugin.Extract()

      _, err := os.Stat(destFolder + "/test.txt");
      Expect(err).To(BeNil())
    })

    It("should extract even if previous archive was downloaded, but not extracted", func() {
      failedAttempt := versionsFolder + "/" + name + "/" + filename

      os.MkdirAll(failedAttempt, 0700)

      plugin.Extract()

      _, err := os.Stat(destFolder + "/test.txt");
      Expect(err).To(BeNil())

      _, err = os.Stat(failedAttempt);
      Expect(err).ShouldNot(BeNil())
    })
  })

  Describe("Download", func() {
    var (
      guard *monkey.PatchGuard
      ts *httptest.Server
    )

    BeforeEach(func() {
      ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        status := 200

        if _, ok := r.URL.Query()["status"]; ok {
          fmt.Sscanf(r.URL.Query().Get("status"), "%d", &status)
        }

        w.WriteHeader(status)
        io.WriteString(w, "test")
      }))

      path, _ = filepath.Abs("../testdata/plugins")
      filename = "node-v5.0.0-darwin-x64.tar.gz"
      archivePath = path + "/" + filename
      destFolder = path + "/" + filename
      url = ts.URL + "/" + filename

      info = map[string]string{
        "name": "node",
        "version": "5.0.0",
        "destination-folder": destFolder,
        "archive-folder": path,
        "archive-path": archivePath,
        "url": url,
      }

      monkey.Patch(variables.Home, func() string {
        return versionsFolder
      })

      var d *Plugin
      ptype := reflect.TypeOf(d)

      guard = monkey.PatchInstanceMethod(ptype, "Info",
        func(*Plugin) (map[string]string, error) {
          return info, nil
        },
      )

      plugin = New("node", "5.0.0")
    })

    AfterEach(func() {
      defer ts.Close()
      guard.Unpatch()
      os.RemoveAll(archivePath)
      os.RemoveAll(destFolder)
    })

    It("returns error if version was not defined", func() {
      plugin := New("node")
      _, err := plugin.Download()
      Expect(err).Should(MatchError("Version was not defined"))
    })

    Describe("200 response", func() {
      It("should download tar", func() {
        plugin.Download()

        _, err := os.Stat(archivePath);
        Expect(err).To(BeNil())
      })

      It("should not download anything if file already exist", func() {
        os.MkdirAll(destFolder, 0777)

        response, _ := plugin.Download()
        Expect(response).To(BeNil())
      })
    })

    Describe("404 response", func() {
      It("should return error", func() {
        info["url"] += "?status=404"
        _, err := New("node", "5.0.0").Download()

        Expect(err).Should(MatchError("Incorrect version 5.0.0"))
      })
    })
  })

  Describe("Info", func() {
    BeforeEach(func() {
      info = map[string]string{
        "name": "node",
        "version": "5.0.0",
        "filename": "node-arch",
        "url": url,
      }

      var d *nodejs.Node
      ptype := reflect.TypeOf(d)

      var guard *monkey.PatchGuard
      guard = monkey.PatchInstanceMethod(ptype, "Info",
        func(*nodejs.Node, string) (map[string]string, error) {
          guard.Unpatch()
          return info, nil
        })

      plugin = New("node", "5.0.0")
    })

    It("returns error if version was not defined", func() {
      plugin := New("node")
      _, err := plugin.Info()
      Expect(err).Should(MatchError("Version was not defined"))
    })

    It("should augment output from plugin `Info` method", func() {
      info, _ := plugin.Info()

      Expect(info["archive-folder"]).To(Equal(os.TempDir()))
      Expect(info["archive-path"]).To(Equal(os.TempDir() + "node-v5.0.0-darwin-x64.tar.gz"))
      Expect(info["destination-folder"]).To(Equal(variables.Home() + "/node/5.0.0"))
    })
  })

  Describe("ComposeVersions", func() {
    It("should compose versions", func() {
      compose := ComposeVersions([]string{"0.8.2", "4.4.7", "6.3.0", "6.4.2"})

      Expect(compose["0.x"]).To(Equal([]string{"0.8.2"}))
      Expect(compose["4.x"]).To(Equal([]string{"4.4.7"}))
      Expect(compose["6.x"]).To(Equal([]string{"6.3.0", "6.4.2"}))
    })
  })

  Describe("GetKeys", func() {
    It("should get version keys", func() {
      list := map[string][]string{"4.x": []string{}, "0.x": []string{"0.8.2"}}
      keys := GetKeys(list)

      Expect(keys[0]).To(Equal("0.x"))
      Expect(keys[1]).To(Equal("4.x"))
    })
  })

  Describe("GetElements", func() {
    It("should get version elements", func() {
      list := ComposeVersions([]string{"0.8.2", "4.4.7", "6.3.0", "6.4.2"})
      elements := GetElements("6.x", list)

      Expect(elements[0]).To(Equal("6.3.0"))
      Expect(elements[1]).To(Equal("6.4.2"))
    })
  })
})
