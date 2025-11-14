//go:build !windows

package main

import "fmt"

func installService() {
	fmt.Println("Error: Service installation is only supported on Windows")
}

func uninstallService() {
	fmt.Println("Error: Service uninstallation is only supported on Windows")
}

func startService() {
	fmt.Println("Error: Service start is only supported on Windows")
}

func stopService() {
	fmt.Println("Error: Service stop is only supported on Windows")
}

func createConfigFile(installDir string) error {
	return fmt.Errorf("config file creation for service is only supported on Windows")
}