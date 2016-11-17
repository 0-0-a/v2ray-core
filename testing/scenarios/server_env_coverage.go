// +build coverage

package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"

	"v2ray.com/core/common/uuid"
)

func BuildV2Ray() error {
	GenTestBinaryPath()
	if _, err := os.Stat(testBinaryPath); err == nil {
		return nil
	}

	cmd := exec.Command("go", "test", "-tags", "json coverage coveragemain", "-coverpkg", "v2ray.com/core/...", "-c", "-o", testBinaryPath, GetSourcePath())
	return cmd.Run()
}

func RunV2Ray(configFile string) *exec.Cmd {
	GenTestBinaryPath()

	covDir := filepath.Join(os.Getenv("GOPATH"), "out", "v2ray", "cov")
	profile := uuid.New().String() + ".out"
	proc := exec.Command(testBinaryPath, "-config", configFile, "-test.run", "TestRunMainForCoverage", "-test.coverprofile", profile, "-test.outputdir", covDir)
	proc.Stderr = os.Stderr
	proc.Stdout = os.Stdout

	return proc
}
