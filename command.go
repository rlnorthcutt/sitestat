package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Defining a new public type 'Command'
type Command struct {
	rootPath    string
	contentDir  string
	outputDir   string
	templateDir string
}

// Defining a global varaiable for Command
var command Command

// **********  Public Command Methods  **********

// Initializes a new Repose project.
// It creates the proper folder structure and starter files.
func (c *Command) Init() string {
	configFile := ConfigFile
	if c.rootPath != "" {
		configFile = filepath.Join(c.rootPath, ConfigFile)
	}

	// Check if the config.yml file already exists
	if filesystem.Exists(configFile) {
		logger.Fatal("Warning: The config file exists at %s. Please choose a new root directory.", configFile)
	}

	// Create the project files
	if err := initCommand.CreateNewProjectFiles(c.rootPath); err != nil {
		logger.Fatal("Error creating site structure: ", err)
	}
	return ""
}

// Creates new content based on the provided content type and filename.
// It requires two arguments: content type and filename.
// The content type defines the path, so it can also include a subfolder
func (c *Command) New(config Config) {
	if len(os.Args) < 4 {
		logger.Warn("Missing arguments. Usage: repose new [CONTENTTYPE] [FILENAME]")
		return
	} else if len(os.Args) > 4 {
		logger.Warn("File name cannot contain spaces. Usage: repose new [CONTENTTYPE] [FILENAME]")
		return
	}

	typeDirectory := os.Args[2]
	fileNameParam := os.Args[3]

	if err := c.createNewContent(config, typeDirectory, fileNameParam); err != nil {
		logger.Error(err.Error())
	}
}

// Generates a new project with demo content and templates to create a new site.
// @TODO: create demo content so this works
func (c *Command) Demo() string {
	logger.Info("Generating demo content")
	return ""
}

// Builds the Repose site based on the current project default values.
// It uses command-line flags to modify the root directory and config file.
// If there is an error parsing the command flags, it prints an error message.
func (c *Command) Build(config Config) {
	logger.Info("Building site from %s with config %s\n", c.rootPath, ConfigFile)
	if err := buildCommand.BuildSite(); err != nil {
		fmt.Println("Error building site:", err)
	}
	logger.Success("Site built successfully")
}

// Starts serving the Repose site for local preview.
func (c *Command) Preview(config Config) {
	logger.Info("Setting up the local preview server")

	// Define the directory to serve.
	if c.rootPath == "" {
		c.rootPath = "."
	}
	webDir := filepath.Join(c.rootPath, config.OutputDirectory)

	// Setup the HTTP server.
	fs := http.FileServer(http.Dir(webDir))
	http.Handle("/", fs)

	// Start the server in a new goroutine so it doesn't block opening the browser.
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Error("Error starting server:", err)
			panic(err)
		}
	}()

	logger.Info("Preview server ready at %s/index.html", config.PreviewURL)
	logger.Plain("Press Ctrl+C to stop the server")

	// Give the server a moment to start.
	time.Sleep(500 * time.Millisecond)

	// Open the browser.
	c.openBrowser(config.PreviewURL + "/index.html")

	// Keep the server running.
	select {}
}

// Updates the Repose binary in the current directory
func (c *Command) Update() string {
	fmt.Printf("Repose update placeholder")
	return ""
}

// Displays the help text for the command-line tool
func (c *Command) Help() string {
	response := HelpText
	logger.Info(response)
	return ""
}

// Set the root path and comomon directories for commands
func (c *Command) SetRootPath(path string) {
	c.rootPath = path
	c.contentDir = filepath.Join(path, config.ContentDirectory)
	c.outputDir = filepath.Join(path, config.OutputDirectory)
	c.templateDir = filepath.Join(path, "template")
}

// **********  Private Command Methods  **********

// processFileName takes a name and returns the filename (with extension) and title.
func (c *Command) processFileName(fileName string) (string, string) {
	// Check the file extension
	ext := filepath.Ext(fileName)
	if ext != ".md" && ext != ".html" {
		// If no extension or any other extension, make it .md
		fileName = fileName + ".md"
		ext = ".md" // Update the extension to .md
	}

	fileNameWithoutExt := strings.TrimSuffix(fileName, ext)

	// Convert fileName to a title
	replaceWithSpaces := strings.Replace(strings.Replace(fileNameWithoutExt, "-", " ", -1), "_", " ", -1)
	caser := cases.Title(language.English)
	title := caser.String(replaceWithSpaces)

	return fileName, title
}

// defaultContent returns default content based on the content type.
func (c *Command) defaultContent(contentType string, title string) string {
	content := NewMD

	// Replace placeholders with actual values
	content = strings.Replace(content, "{title}", title, -1)
	content = strings.Replace(content, "{contentType}", contentType, -1)
	content = strings.Replace(content, "{author}", config.Author, -1)

	return content
}

// createNewContent creates a new content file in the specified directory.
// @TODO refactor this and break into 2-3 methods
func (c *Command) createNewContent(config Config, typeDirectory, fileNameParam string) error {
	fileName, title := c.processFileName(fileNameParam)

	// Determine content type
	contentType := typeDirectory
	if strings.Contains(typeDirectory, "/") {
		contentType = strings.Split(typeDirectory, "/")[0]
	}

	// Construct the path
	logger.Info("Creating new %s in %s", contentType, config.ContentDirectory)
	path := filepath.Join(config.ContentDirectory, typeDirectory, fileName)

	// Get default content
	content := c.defaultContent(contentType, title)

	// Create the file or directory
	if err := filesystem.Create(path, content); err != nil {
		return fmt.Errorf("failed to create %s: %v", path, err)
	}

	logger.Success("Successfully created new %s: %s", contentType, path)

	// Check if the template exists
	templateName := contentType + ".tmpl"
	found, err := filesystem.ExistsRecursive(templateName, "template")
	if err != nil {
		fmt.Println("Error searching for template:", err)
		return nil
	}

	// Ask the user to create the template file if it doesn't exist
	if !found {
		logger.Warn("Template file not found: %s", templateName)
		logger.Plain("Do you want to create this template? (Yes/no)")

		// Read the user's response
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Error reading response:", err)
			return nil
		}

		// Trim whitespace and newline character
		response = strings.ToLower(strings.TrimSpace(response))
		// If yes, then create the template file
		if response == "yes" || response == "" {
			logger.Info("Creating template file: %s", templateName)
			path := "template/" + templateName
			template := "DefaultTemplate_" + config.Theme
			if err := filesystem.Create(path, template); err != nil {
				logger.Error("Error creating template:", err)
				return nil
			}
			logger.Success("Template created successfully.")
		}
	}

	// Check if the editor is set and not empty, then open the file with it
	if config.Editor != "" && config.Editor != "none" {
		if err := c.openFileInEditor(config.Editor, path); err != nil {
			// Log the error but do not fail the entire operation
			logger.Error("Failed to open file in editor: %v", err)
		}
	}

	return nil
}

// openFileInEditor opens the specified file in the given editor.
func (c *Command) openFileInEditor(editor, filePath string) error {
	logger.Plain("Opening file in editor: %s", editor)
	// Pause for a moment before opening the editor
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Helper function to prompt for input in a standard way
func (c *Command) promptForInput(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	// Use this instead of logger.Info to avoid the newline character
	fmt.Printf("------- %s [%s]: ", prompt, defaultValue)

	input, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Error reading input:", err)
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

// openBrowser tries to open the browser with a given URL.
func (c *Command) openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		logger.Error("Failed to open the browser: %v\n", err)
	}
}
