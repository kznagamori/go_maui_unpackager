package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/beevik/etree"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path-to-csproj>")
		return
	}

	csprojPath := os.Args[1]

	// .csprojファイルの解析
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(csprojPath); err != nil {
		fmt.Printf("Error reading csproj file: %v\n", err)
		return
	}

	project := doc.SelectElement("Project")
	if project == nil {
		fmt.Println("No <Project> element found in the csproj file.")
		return
	}

	// <PropertyGroup>に<WindowsPackageType>None</WindowsPackageType>を追加
	propertyGroups := project.SelectElements("PropertyGroup")
	for _, propertyGroup := range propertyGroups {
		if propertyGroup.SelectElement("WindowsPackageType") == nil {
			windowsPackageType := propertyGroup.CreateElement("WindowsPackageType")
			windowsPackageType.SetText("None")
		}
	}

	// 変更を保存 (インデントを設定)
	doc.Indent(2)
	if err := doc.WriteToFile(csprojPath); err != nil {
		fmt.Printf("Error writing csproj file: %v\n", err)
		return
	}

	// launchSettings.jsonの解析と変更
	propertiesDir := filepath.Join(filepath.Dir(csprojPath), "Properties")
	launchSettingsPath := filepath.Join(propertiesDir, "launchSettings.json")

	launchSettingsFile, err := os.Open(launchSettingsPath)
	if err != nil {
		fmt.Printf("Error reading launchSettings.json: %v\n", err)
		return
	}
	defer launchSettingsFile.Close()

	var launchSettingsLines []string
	scanner := bufio.NewScanner(launchSettingsFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"commandName":`) {
			line = `      "commandName": "Project",`
		}
		launchSettingsLines = append(launchSettingsLines, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading launchSettings.json: %v\n", err)
		return
	}

	launchSettingsFile.Close()

	// 変更を保存
	launchSettingsFile, err = os.Create(launchSettingsPath)
	if err != nil {
		fmt.Printf("Error writing launchSettings.json: %v\n", err)
		return
	}
	defer launchSettingsFile.Close()

	writer := bufio.NewWriter(launchSettingsFile)
	for _, line := range launchSettingsLines {
		fmt.Fprintln(writer, line)
	}
	writer.Flush()

	// .csprojファイルからターゲットフレームワークを抽出
	targetFramework := ""
	frameworkRegex := regexp.MustCompile(`net\d+\.\d+-windows\d+\.\d+\.\d+\.\d+`)

	for _, propertyGroup := range propertyGroups {
		for _, element := range propertyGroup.ChildElements() {
			if element.Tag == "TargetFrameworks" && element.SelectAttrValue("Condition", "") != "" {
				matches := frameworkRegex.FindAllString(element.Text(), -1)
				for _, match := range matches {
					if strings.Contains(match, "windows") {
						targetFramework = match
						break
					}
				}
			}
		}
	}

	if targetFramework == "" {
		fmt.Println("No valid TargetFramework found in the csproj file.")
		return
	}

	// バッチファイルの作成
	batchFilePath := filepath.Join(filepath.Dir(csprojPath), "unpackaged_app_publish.bat")
	batchFileContent := fmt.Sprintf(`dotnet publish -f %s -c Release -p:RuntimeIdentifierOverride=win10-x64 -p:WindowsPackageType=None -p:WindowsAppSDKSelfContained=true`, targetFramework)

	if err := os.WriteFile(batchFilePath, []byte(batchFileContent), 0644); err != nil {
		fmt.Printf("Error writing batch file: %v\n", err)
		return
	}

	fmt.Println("MauiUnpackager completed successfully.")
}
