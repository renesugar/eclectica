package nodejs_test

import (
  "regexp"
  "io/ioutil"
  "runtime"
  "net/http"
  "net/http/httptest"
  "io"

  "github.com/jarcoal/httpmock"
  . "github.com/onsi/ginkgo"
  . "github.com/onsi/gomega"

  ."github.com/markelog/eclectica/plugins/nodejs"
)

func Read(path string) string {
  bytes, _ := ioutil.ReadFile(path)

  return string(bytes)
}

var _ = Describe("nodejs", func() {
  var (
    remotes []string
    err error
  )

  node := &Node{}

  Describe("ListRemote", func() {
    old := VersionsLink

    AfterEach(func() {
      VersionsLink = old
    })

    Describe("success", func() {
      BeforeEach(func() {

        content := Read("../../testdata/nodejs/dist.html")

        // httpmock is not incompatible with goquery :/.
        // See https://github.com/jarcoal/httpmock/issues/18
        req, _ := http.NewRequest("GET", VersionsLink, nil)

        rr := httptest.NewRecorder()
        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(http.StatusOK)
          w.Header().Set("Content-Type", "text/html")
          io.WriteString(w, content)
        })

        handler.ServeHTTP(rr, req)

        remotes, err = node.ListRemote()
      })

      It("should not return an error", func() {
        Expect(err).To(BeNil())
      })

      It("should have correct version values", func() {
        rp := regexp.MustCompile("[[:digit:]]+\\.[[:digit:]]+\\.[[:digit:]]+$")

        for _, element := range remotes {
          Expect(rp.MatchString(element)).To(Equal(true))
        }
      })
    })

    Describe("fail", func() {
      BeforeEach(func() {
        VersionsLink = ""
        remotes, err = node.ListRemote()
      })

      It("should return an error", func() {
        Expect(err).Should(MatchError("Can't establish connection"))
      })
    })
  })

  Describe("Info", func() {
    BeforeEach(func() {
      content := Read("../../testdata/nodejs/latest.txt")

      httpmock.Activate()

      httpmock.RegisterResponder(
        "GET",
        "https://nodejs.org/dist/latest/SHASUMS256.txt",
        httpmock.NewStringResponder(200, content),
      )

      httpmock.RegisterResponder(
        "GET",
        "https://nodejs.org/dist/lts/SHASUMS256.txt",
        httpmock.NewStringResponder(200, content),
      )
    })

    AfterEach(func() {
      defer httpmock.DeactivateAndReset()
    })

    It("should get info about latest version", func() {
      result, _ := node.Info("latest")

      Expect(result["name"]).To(Equal("node"))
      Expect(result["version"]).To(Equal("6.3.1"))

      // :/
      if runtime.GOOS == "darwin" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-darwin-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/latest/node-v6.3.1-darwin-x64.tar.gz"))
      } else if runtime.GOOS == "linux" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-linux-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/latest/node-v6.3.1-linux-x64.tar.gz"))
      }
    })

    It("should get info about lts version", func() {
      result, _ := node.Info("lts")

      Expect(result["name"]).To(Equal("node"))
      Expect(result["version"]).To(Equal("6.3.1"))

      // :/
      if runtime.GOOS == "darwin" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-darwin-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/lts/node-v6.3.1-darwin-x64.tar.gz"))
      } else if runtime.GOOS == "linux" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-linux-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/lts/node-v6.3.1-linux-x64.tar.gz"))
      }
    })

    It("should get info about 6.3.1 version", func() {
      result, _ := node.Info("6.3.1")

      Expect(result["name"]).To(Equal("node"))
      Expect(result["version"]).To(Equal("6.3.1"))

      // :/
      if runtime.GOOS == "darwin" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-darwin-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/v6.3.1/node-v6.3.1-darwin-x64.tar.gz"))
      } else if runtime.GOOS == "linux" {
        Expect(result["filename"]).To(Equal("node-v6.3.1-linux-x64"))
        Expect(result["url"]).To(Equal("https://nodejs.org/dist/v6.3.1/node-v6.3.1-linux-x64.tar.gz"))
      }
    })
  })
})
