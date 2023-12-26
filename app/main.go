//go:build linux
// +build linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"
	// Uncomment this block to pass the first stage!

	"net/http"
	"os"
	"os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	image := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	//fmt.Println("main", image, command, args)
	// create a jail dir
	JailDir, _ := createRootDir()
	token, err := getToken(image)
	must(err)
	manifest := getManifest(image, token)
	// download and extract layers
	downloadAndExtract([]struct {
		MediaType string
		Size      int
		Digest    string
	}(manifest.Layers), image, token, JailDir)
	//err2 := ls(JailDir)
	//must(err2)
	//copyFunc(command, JailDir)
	// execute function in new root
	runCommandInSandBox(command, JailDir, args)
}

func ls(JailDir string) error {
	cmd := exec.Command("ls", JailDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err2 := cmd.Run()
	return err2
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func runCommandInSandBox(command string, JailDir string, args []string) {
	chrootArgs := []string{JailDir, command}
	chrootArgs = append(chrootArgs, args...)
	cmd := exec.Command("chroot", chrootArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		func() {
			var exitError *exec.ExitError
			errors.As(err, &exitError)
			if exitError.ExitCode() != 0 {
				os.Exit(exitError.ExitCode())
			}
		}()
	}
}

func copyFunc(command string, JailDir string) {
	mkdirP := exec.Command("mkdir", "-p", JailDir+"/usr/local/bin")
	mkdirP.Stdout = os.Stdout
	mkdirP.Stderr = os.Stderr
	pErr := mkdirP.Run()
	if pErr != nil {
		fmt.Println("mkdir -p failure", pErr)
	}
	copyCommand := exec.Command("cp", command, JailDir+"/usr/local/bin/")
	copyCommand.Stdin = os.Stdin
	copyCommand.Stderr = os.Stderr
	copyCommand.Stdout = os.Stdout
	err := copyCommand.Run()
	if err != nil {
		fmt.Println("Not able to copy command", command)
	}
}

func createRootDir() (string, bool) {
	JailDir := "jailDir"
	command := exec.Command("mkdir", "-p", JailDir)
	err := command.Run()
	if err != nil {
		fmt.Println("failed to create directory", err)
		return "", false
	}
	return JailDir, true
}

func downloadAndExtract(layers []struct {
	MediaType string
	Size      int
	Digest    string
}, image, token, jailDir string) {
	client := http.Client{}
	layer := layers[0]
	req, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/blobs/%s", image, layer.Digest), nil)
	must(err)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := client.Do(req)
	resp, err := io.ReadAll(response.Body)
	must(err)
	fileName := "image.tar"
	file, err := os.OpenFile(fileName, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0655)
	_, err = file.Write(resp)
	//ls(".")
	must(err)
	exec.Command("tar", "-xf", fileName, "-C", jailDir).Run()
	must(err)

}
func getManifest(image, token string) Manifest {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/latest", image), nil)
	must(err)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	//fmt.Println(req)
	res, err := http.DefaultClient.Do(req)
	must(err)
	defer res.Body.Close()
	return jsonToManifest(res.Body)
}

func getToken(image string) (string, error) {
	res, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", image))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.Token, nil
}

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

func jsonToManifest(body io.ReadCloser) Manifest {
	var manifest Manifest
	jsonBody, err := io.ReadAll(body)
	must(err)
	err = json.Unmarshal(jsonBody, &manifest)
	must(err)
	return manifest
}
