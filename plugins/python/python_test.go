package python_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	eio "github.com/markelog/eclectica/io"
	. "github.com/markelog/eclectica/plugins/python"
	"github.com/markelog/eclectica/variables"
)

var _ = Describe("python", func() {
	var (
		remotes []string
		err     error
	)

	python := &Python{}

	Describe("ListRemote", func() {
		old := VersionLink

		AfterEach(func() {
			VersionLink = old
		})

		Describe("success", func() {
			BeforeEach(func() {
				content := eio.Read("../../testdata/plugins/python/index.html")

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					status := 200

					if _, ok := r.URL.Query()["status"]; ok {
						fmt.Sscanf(r.URL.Query().Get("status"), "%d", &status)
					}

					w.WriteHeader(status)
					io.WriteString(w, content)
				}))

				VersionLink = ts.URL

				remotes, err = python.ListRemote()
			})

			It("should not return an error", func() {
				Expect(err).To(BeNil())
			})

			It("should exclude some version", func() {
				Expect(remotes[0]).To(Equal("3.4.5"))

				last := len(remotes) - 1
				Expect(remotes[last]).To(Equal("3.0"))
			})
		})

		Describe("fail", func() {
			BeforeEach(func() {
				VersionLink = ""
				remotes, err = python.ListRemote()
			})

			It("should return an error", func() {
				Expect(err).Should(MatchError(variables.ConnectionError))
			})
		})
	})

	Describe("Info", func() {
		BeforeEach(func() {
			content := eio.Read("../../testdata/plugins/python/index.html")

			httpmock.Activate()

			httpmock.RegisterResponder(
				"GET",
				"https://pythonjs.org/dist/latest/SHASUMS256.txt",
				httpmock.NewStringResponder(200, content),
			)

			httpmock.RegisterResponder(
				"GET",
				"https://pythonjs.org/dist/lts/SHASUMS256.txt",
				httpmock.NewStringResponder(200, content),
			)
		})

		AfterEach(func() {
			defer httpmock.DeactivateAndReset()
		})

		It("should get info about rc version", func() {
			result := (&Python{Version: "2.7.13-rc1"}).Info()

			Expect(result["version"]).To(Equal("2.7.13rc1"))
			Expect(result["filename"]).To(Equal("Python-2.7.13rc1"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/2.7.13/Python-2.7.13rc1.tgz"))
		})

		It("should get info about rc version with nil at the end", func() {
			result := (&Python{Version: "2.7.0-rc1"}).Info()

			Expect(result["version"]).To(Equal("2.7rc1"))
			Expect(result["filename"]).To(Equal("Python-2.7rc1"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/2.7/Python-2.7rc1.tgz"))
		})

		It("should get info about 2.0 version", func() {
			result := (&Python{Version: "3.0.0"}).Info()

			Expect(result["version"]).To(Equal("3.0"))
			Expect(result["filename"]).To(Equal("Python-3.0"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/3.0/Python-3.0.tgz"))
		})

		It("should get info about 3.0 version", func() {
			result := (&Python{Version: "3.0.0"}).Info()

			Expect(result["version"]).To(Equal("3.0"))
			Expect(result["filename"]).To(Equal("Python-3.0"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/3.0/Python-3.0.tgz"))
		})

		It("up the not ante for 3.2", func() {
			result := (&Python{Version: "3.2.0"}).Info()

			Expect(result["version"]).To(Equal("3.2"))
			Expect(result["filename"]).To(Equal("Python-3.2"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/3.2/Python-3.2.tgz"))
		})

		It("up the ante for 3.3", func() {
			result := (&Python{Version: "3.3.0"}).Info()

			Expect(result["version"]).To(Equal("3.3.0"))
			Expect(result["filename"]).To(Equal("Python-3.3.0"))
			Expect(result["url"]).To(Equal("https://www.python.org/ftp/python/3.3.0/Python-3.3.0.tgz"))
		})
	})
})
