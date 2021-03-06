package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("bbl up", func() {
	var (
		tempDirectory              string
		serviceAccountKeyPath      string
		pathToFakeTerraform        string
		pathToTerraform            string
		fakeTerraformBackendServer *httptest.Server
		fakeBOSHServer             *httptest.Server
		fakeBOSH                   *fakeBOSHDirector
	)

	BeforeEach(func() {
		var err error
		fakeBOSH = &fakeBOSHDirector{}
		fakeBOSHServer = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			fakeBOSH.ServeHTTP(responseWriter, request)
		}))

		fakeTerraformBackendServer = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/output/external_ip":
				responseWriter.Write([]byte("127.0.0.1"))
			case "/output/director_address":
				responseWriter.Write([]byte(fakeBOSHServer.URL))
			}
		}))

		pathToFakeTerraform, err = gexec.Build("github.com/cloudfoundry/bosh-bootloader/bbl/faketerraform",
			"--ldflags", fmt.Sprintf("-X main.backendURL=%s", fakeTerraformBackendServer.URL))
		Expect(err).NotTo(HaveOccurred())

		pathToTerraform = filepath.Join(filepath.Dir(pathToFakeTerraform), "terraform")
		err = os.Rename(pathToFakeTerraform, pathToTerraform)
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("PATH", strings.Join([]string{filepath.Dir(pathToTerraform), os.Getenv("PATH")}, ":"))

		tempDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		tempFile, err := ioutil.TempFile("", "gcpServiceAccountKey")
		Expect(err).NotTo(HaveOccurred())

		serviceAccountKeyPath = tempFile.Name()
		err = ioutil.WriteFile(serviceAccountKeyPath, []byte(serviceAccountKey), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
	})

	It("writes iaas to state", func() {
		args := []string{
			"--state-dir", tempDirectory,
			"up",
			"--iaas", "gcp",
			"--gcp-service-account-key", serviceAccountKeyPath,
			"--gcp-project-id", "some-project-id",
			"--gcp-zone", "some-zone",
			"--gcp-region", "us-west1",
		}

		executeCommand(args, 0)

		state := readStateJson(tempDirectory)
		Expect(state.IAAS).To(Equal("gcp"))
	})

	Context("when providing iaas via env vars", func() {
		BeforeEach(func() {
			err := os.Setenv("BBL_IAAS", "gcp")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Unsetenv("BBL_IAAS")
			Expect(err).NotTo(HaveOccurred())
		})

		It("writes iaas to state", func() {
			args := []string{
				"--state-dir", tempDirectory,
				"up",
				"--gcp-service-account-key", serviceAccountKeyPath,
				"--gcp-project-id", "some-project-id",
				"--gcp-zone", "some-zone",
				"--gcp-region", "us-west1",
			}

			executeCommand(args, 0)

			state := readStateJson(tempDirectory)
			Expect(state.IAAS).To(Equal("gcp"))
		})
	})

	It("exits 1 and prints error message when --iaas is not provided", func() {
		session := executeCommand([]string{"--state-dir", tempDirectory, "up"}, 1)
		Expect(session.Err.Contents()).To(ContainSubstring("--iaas [gcp, aws] must be provided"))
	})

	It("exits 1 and prints error message when unsupported --iaas is provided", func() {
		args := []string{
			"--state-dir", tempDirectory,
			"up",
			"--iaas", "bad-iaas-value",
		}

		session := executeCommand(args, 1)
		Expect(session.Err.Contents()).To(ContainSubstring(`"bad-iaas-value" is an invalid iaas type, supported values are: [gcp, aws]`))
	})
})
