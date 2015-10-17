package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/v2ray/v2ray-core/tools/git"
)

var (
	targetOS   = flag.String("os", runtime.GOOS, "Target OS of this build.")
	targetArch = flag.String("arch", runtime.GOARCH, "Target CPU arch of this build.")
	archive    = flag.Bool("zip", false, "Whether to make an archive of files or not.")
)

func createTargetDirectory(version string, goOS GoOS, goArch GoArch) (string, error) {
	suffix := "-custom"
	if version != "custom" {
		suffix = getSuffix(goOS, goArch)
	}
	GOPATH := os.Getenv("GOPATH")

	targetDir := filepath.Join(GOPATH, "bin", "v2ray"+suffix)
	if version != "custom" {
		os.RemoveAll(targetDir)
	}
	err := os.MkdirAll(targetDir, os.ModeDir|0777)
	return targetDir, err
}

func getTargetFile(goOS GoOS) string {
	suffix := ""
	if goOS == "Windows" {
		suffix += ".exe"
	}
	return "v2ray" + suffix
}

func main() {
	flag.Parse()
	fmt.Println(os.Args)

	v2rayOS := parseOS(*targetOS)
	v2rayArch := parseArch(*targetArch)

	version, err := git.RepoVersionHead()
	if version == git.VersionUndefined {
		version = "custom"
	}
	if err != nil {
		fmt.Println("Unable to detect V2Ray version: " + err.Error())
		return
	}
	fmt.Printf("Building V2Ray (%s) for %s %s\n", version, v2rayOS, v2rayArch)
	version = "v1.0"

	targetDir, err := createTargetDirectory(version, v2rayOS, v2rayArch)
	if err != nil {
		fmt.Println("Unable to create directory " + targetDir + ": " + err.Error())
	}

	targetFile := getTargetFile(v2rayOS)
	err = buildV2Ray(filepath.Join(targetDir, targetFile), version, v2rayOS, v2rayArch)
	if err != nil {
		fmt.Println("Unable to build V2Ray: " + err.Error())
	}

	err = copyConfigFiles(targetDir, v2rayOS)
	if err != nil {
		fmt.Println("Unable to copy config files: " + err.Error())
	}
}
