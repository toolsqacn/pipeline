/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"

	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
)

var (
	url      = flag.String("url", "", "The url of the Git repository to initialize.")
	revision = flag.String("revision", "", "The Git revision to make the repository HEAD")
	path     = flag.String("path", "", "Path of directory under which git repository will be copied")
)

func run(logger *zap.SugaredLogger, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	if err := c.Run(); err != nil {
		logger.Errorf("Error running %v %v: %v\n%v", cmd, args, err, output.String())
		return err
	}
	return nil
}

func runOrFail(logger *zap.SugaredLogger, cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output

	if err := c.Run(); err != nil {
		logger.Fatalf("Unexpected error running %v %v: %v\n%v", cmd, args, err, output.String())
	}
}

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "git-init")
	defer logger.Sync()

	// HACK HACK HACK
	// Git seems to ignore $HOME/.ssh and look in /root/.ssh for unknown reasons.
	// As a workaround, symlink /root/.ssh to where we expect the $HOME to land.
	// This means SSH auth only works for our built-in git support, and not
	// custom steps.
	err := os.Symlink("/builder/home/.ssh", "/root/.ssh")
	if err != nil {
		logger.Fatalf("Unexpected error creating symlink: %v", err)
	}
	if *revision == "" {
		*revision = "master"
	}
	if *path != "" {
		runOrFail(logger, "git", "init", *path)
		if _, err := os.Stat(*path); os.IsNotExist(err) {
			if err := os.Mkdir(*path, os.ModePerm); err != nil {
				logger.Debugf("Creating directory at path %s", *path)
			}
		}
		if err := os.Chdir(*path); err != nil {
			logger.Fatalf("Failed to change directory with path %s; err %v", path, err)
		}
	} else {
		runOrFail(logger, "git", "init")
	}

	runOrFail(logger, "git", "remote", "add", "origin", *url)
	if err := run(logger, "git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *revision); err != nil {
		// Fetch can fail if an old commitid was used so try git pull, performing regardless of error
		// as no guarantee that the same error is returned by all git servers gitlab, github etc...
		if err := run(logger, "git", "pull", "--recurse-submodules=yes", "origin"); err != nil {
			logger.Warnf("Failed to pull origin : %s", err)
		}
		runOrFail(logger, "git", "checkout", *revision)
	} else {
		runOrFail(logger, "git", "reset", "--hard", "FETCH_HEAD")
	}

	logger.Infof("Successfully cloned %q @ %q in path %q", *url, *revision, *path)
}
