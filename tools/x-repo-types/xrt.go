// Copyright (C) 2019-2023 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	_ "embed"
)

//go:embed typeAnalyzer/main.tmpl
var differTmpl string

func main() {
	var xPkg, xBranch, xType, yPkg, yBranch, yType string

	rootCmd := &cobra.Command{
		Use:   "xrt",
		Short: "Compare types across repos",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runApp(xPkg, xBranch, xType, yPkg, yBranch, yType); err != nil {
				log.Fatal(err)
			}
		},
	}

	rootCmd.Flags().StringVar(&xPkg, "x-package", "", "Go repo and package for type x")
	rootCmd.Flags().StringVar(&xBranch, "x-branch", "", "repository branch for type x")
	rootCmd.Flags().StringVar(&xType, "x-type", "", "Exported type in the package for type x")
	rootCmd.Flags().StringVar(&yPkg, "y-package", "", "Go repo and package for type for type y")
	rootCmd.Flags().StringVar(&yBranch, "y-branch", "", "repository branch for type y")
	rootCmd.Flags().StringVar(&yType, "y-type", "", "Exported type in the package for type y")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func backupFile(src string) (string, error) {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return "", err
	}

	tmpFile, err := ioutil.TempFile("", "backup-*")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(tmpFile.Name(), content, 0644)
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func findPkgRoot() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func setUp() (map[string]string, error) {
	pkgRoot, err := findPkgRoot()
	if err != nil {
		return nil, err
	}

	goModPath := filepath.Join(pkgRoot, "go.mod")
	goSumPath := filepath.Join(pkgRoot, "go.sum")
	localMainPath := filepath.Join("typeAnalyzer", "main.go")

	backups := make(map[string]string)
	for _, path := range []string{goModPath, goSumPath, localMainPath} {
		backup, err := backupFile(path)
		if err != nil {
			return nil, err
		}
		backups[backup] = path
	}
	return backups, nil
}

func restoreFile(src, dst string) error {
	// assuming that dst already exists
	dstFileInfo, err := os.Stat(dst)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, content, dstFileInfo.Mode())
	if err != nil {
		return err
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}

func tearDown(fileBackups map[string]string) error {
	for backup, path := range fileBackups {
		err := restoreFile(backup, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func runApp(xPkg, xBranch, xType, yPkg, yBranch, yType string) (err error) {
	fileBackups, err := setUp()
	fmt.Printf("fileBackups: %#v\n\n", fileBackups)
	if err != nil {
		return err
	}
	defer func() {
		fmt.Printf("tearDown to restore: %#v\n\n", fileBackups)
		err = tearDown(fileBackups)
		if err != nil {
			fmt.Printf("problem during tearDown: %v\n", err)
		}
	}()

	if xPkg == "" || xType == "" {
		return fmt.Errorf("package:%s, and type:%s flags are required", xPkg, xType)
	}
	if yPkg == "" || yType == "" {
		return fmt.Errorf("package:%s, and type:%s flags are required", yPkg, yType)
	}

	xPkgBranch := xPkg
	if xBranch != "" {
		xPkgBranch += "@" + xBranch
	}
	yPkgBranch := yPkg
	if yBranch != "" {
		yPkgBranch += "@" + yBranch
	}

	err = goGet(xPkgBranch)
	if err != nil {
		return err
	}
	err = goGet(yPkgBranch)
	if err != nil {
		return err
	}

	xParts := strings.Split(xPkg, "/")
	yParts := strings.Split(yPkg, "/")

	xRepo := strings.Join(xParts[:3], "/")
	yRepo := strings.Join(yParts[:3], "/")

	xPkgSuffix := strings.Join(xParts[3:], "/")
	yPkgSuffix := strings.Join(yParts[3:], "/")

	// Instantiate the type in a separate process as a "Smoke Test"
	err = instantiate(xRepo, xPkgSuffix, xType)
	if err != nil {
		return err
	}
	err = instantiate(yRepo, yPkgSuffix, yType)
	if err != nil {
		return err
	}

	// Compare the types by running the template typeAnalyzer/main.tmpl in a separate process
	// typeAnalyzer/main will return an error if the types are not the same
	// here we propagate the error to the caller, so as to fail the test.
	err = serializationDiff(xRepo, xPkgSuffix, xType, yRepo, yPkgSuffix, yType)
	if err != nil {
		return err
	}
	return nil
}

func goGet(repo string) error {
	fmt.Println("Downloading repo:", repo)
	cmd := exec.Command("go", "get", repo)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func instantiate(repo, pkgPath, typeName string) error {
	fmt.Println("Instantiating type for:", typeName)

	pkgParts := strings.Split(pkgPath, "/")
	pkgOnly := pkgParts[len(pkgParts)-1]

	code := fmt.Sprintf(`package main

import (
	"fmt"
	"%s/%s"
)

func main() {
	var item %s.%s
	fmt.Printf("Instantiated: %%#v\n\n", item)
}
`, repo, pkgPath, pkgOnly, typeName)

	tmpDir, err := os.MkdirTemp(".", "instantiate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(tmpFile, []byte(code), 0644)
	if err != nil {
		return err
	}

	//nolint:gosec // tmpFile is defined above so no security concerns here
	cmd := exec.Command("go", "run", tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func serializationDiff(xRepo, xPkgPath, xType, yRepo, yPkgPath, yType string) error {
	fmt.Printf("Diffing %s from package %s VS %s from package %s...\n", xType, xPkgPath, yType, yPkgPath)

	tmpl, err := template.New("code").Parse(differTmpl)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"XModulePath":   xRepo,
		"XPackagePath":  xPkgPath,
		"XTypeInstance": xType,
		"YModulePath":   yRepo,
		"YPackagePath":  yPkgPath,
		"YTypeInstance": yType,
	})
	if err != nil {
		fmt.Println("Error executing template:", err)
		os.Exit(1)
	}

	main := filepath.Join("typeAnalyzer", "main.go")
	typeAnalyzer := filepath.Join("typeAnalyzer", "typeAnalyzer.go")
	err = os.WriteFile(main, buf.Bytes(), 0644)
	if err != nil {
		return err
	}

	//nolint:gosec // main and typeAnalyzer are hard-coded above so no security concerns here
	cmd := exec.Command("go", "run", main, typeAnalyzer)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
