package bin_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRuby(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ruby Bin Suite")
}
