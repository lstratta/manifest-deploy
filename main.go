package main

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	// accept a arg that is the manifest dir
	arg := os.Args[1]

	// check that dir exists
	argInfo, err := os.Stat(arg)
	if err != nil {
		log.Fatal(err.Error())
	}
	if !argInfo.IsDir() {
		log.Fatalf("%s is not a directory", arg)
	}

	// read dir
	entries, err := os.ReadDir(arg)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println(entries)

	// generate random passwords & save to secrets
	log.Println("about to generate passwords")
	if err = iterateDir(arg, entries, passwordGen); err != nil {
		log.Fatal(err.Error())
	}

	// create sealed secrets

	// log.Println(argInfo.Name())

	// delete plain text secrets
}

func iterateDir(arg string, entries []fs.DirEntry, modFunc modifier) error {
	log.Println("iterating over dir")
	for _, ent := range entries {
		if ent.IsDir() {
			lEntries, err := os.ReadDir(ent.Name())
			if err != nil {
				log.Fatal(err.Error())
			}
			iterateDir(arg, lEntries, modFunc)
		}

		log.Println("calling modifier function")
		err := modFunc(arg, ent)
		if err != nil {
			return err
		}

		// cmd := exec.Command("kubectl", "apply", "-f", ent.Name())
	}
	return nil
}

type modifier func(string, fs.DirEntry) error

type yamlSecrets struct {
	ApiVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   map[string]any `yaml:"metadata"`
	Data       map[string]any `yaml:"data"`
}

func passwordGen(arg string, ent fs.DirEntry) error {
	// exit out of this function if the file is not sec-template
	if ent.Name() != "sec-template.yaml" {
		return nil
	}

	yamlFile := yamlSecrets{}

	path := filepath.Join(arg, "sec-template.yaml")

	yamlBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal(yamlBytes, &yamlFile); err != nil {
		log.Fatal(err)
	}

	log.Printf("yamlFile: %+v", yamlFile)

	if yamlFile.Kind != "Secret" {
		return errors.New("yaml file is not a kubernetes secret")
	}

	var secretVals []string

	// generate random values
	for i := 0; i < len(yamlFile.Data); i++ {
		output, err := exec.Command("openssl", "rand", "-hex", "16").Output()
		if err != nil {
			return err
		}

		cmd := exec.Command("base64")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, string(output))
		}()

		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		secretVals = append(secretVals, string(out))
	}

	x := 0
	for k := range yamlFile.Data {
		yamlFile.Data[k] = secretVals[x]
		x++
	}

	yamlBytes, err = yaml.Marshal(yamlFile)
	if err != nil {
		return err
	}

	if err = os.WriteFile(path, yamlBytes, os.ModePerm); err != nil {
		return err
	}

	return nil
}
