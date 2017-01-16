package nodejs_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/bouk/monkey"
	"github.com/jarcoal/httpmock"

	eio "github.com/markelog/eclectica/io"
	. "github.com/markelog/eclectica/plugins/nodejs"
)

var _ = Describe("nodejs", func() {
	var (
		remotes []string
		err     error
	)

	node := &Node{}

	Describe("PostInstall", func() {
		var (
			CreateDirFirst  = ""
			WriteFileFirst  = ""
			WriteFileSecond = ""
		)

		BeforeEach(func() {
			monkey.Patch(eio.CreateDir, func(path string) (string, error) {
				CreateDirFirst = path
				return path, nil
			})

			monkey.Patch(eio.WriteFile, func(path, text string) error {
				WriteFileFirst = path
				WriteFileSecond = text
				return nil
			})
		})

		AfterEach(func() {
			CreateDirFirst = ""
			WriteFileFirst = ""
			WriteFileSecond = ""

			monkey.Unpatch(eio.CreateDir)
			monkey.Unpatch(eio.WriteFile)
		})

		It("should correctly creates 'etc' dir", func() {
			(&Node{Version: "1"}).PostInstall()

			Expect(strings.Contains(CreateDirFirst, "versions/node/1/etc")).To(Equal(true))
		})

		It("should writes npmrc with prefix data", func() {
			(&Node{Version: "1"}).PostInstall()

			Expect(strings.Contains(WriteFileFirst, "versions/node/1/etc/npmrc")).To(Equal(true))

			Expect(strings.Contains(WriteFileSecond, "prefix=")).To(Equal(true))
			Expect(strings.Contains(WriteFileSecond, "\n")).To(Equal(true))
			Expect(strings.Contains(WriteFileSecond, ".eclectica/shared")).To(Equal(true))
		})
	})

	Describe("ListRemote", func() {
		old := VersionLink

		AfterEach(func() {
			VersionLink = old
		})

		Describe("success", func() {
			BeforeEach(func() {
				content := eio.Read("../../testdata/plugins/nodejs/dist.html")

				// httpmock is not incompatible with goquery :/.
				// See https://github.com/jarcoal/httpmock/issues/18
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					status := 200

					if _, ok := r.URL.Query()["status"]; ok {
						fmt.Sscanf(r.URL.Query().Get("status"), "%d", &status)
					}

					w.WriteHeader(status)
					io.WriteString(w, content)
				}))

				VersionLink = ts.URL

				remotes, err = node.ListRemote()
			})

			It("should not return an error", func() {
				Expect(err).To(BeNil())
			})

			It("should have correct version values", func() {
				Expect(remotes[0]).To(Equal("6.4.0"))
			})
		})

		Describe("fail", func() {
			BeforeEach(func() {
				VersionLink = ""
				remotes, err = node.ListRemote()
			})

			It("should return an error", func() {
				Expect(err).Should(MatchError("Can't establish connection"))
			})
		})
	})

	Describe("Info", func() {
		BeforeEach(func() {
			content := eio.Read("../../testdata/plugins/nodejs/latest.txt")

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
			Skip("Waiting on #10")
			result := (&Node{Version: "latest"}).Info()

			// :/
			if runtime.GOOS == "darwin" {
				Expect(result["filename"]).To(Equal("node-v6.3.1-darwin-x64"))
				Expect(result["url"]).To(Equal("https://nodejs.org/dist/latest/node-v6.3.1-darwin-x64.tar.gz"))
			} else if runtime.GOOS == "linux" {
				Expect(result["filename"]).To(Equal("node-v6.3.1-linux-x64"))
				Expect(result["url"]).To(Equal("https://nodejs.org/dist/latest/node-v6.3.1-linux-x64.tar.gz"))
			}
		})

		It("should get info about 6.3.1 version", func() {
			result := (&Node{Version: "6.3.1"}).Info()

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
