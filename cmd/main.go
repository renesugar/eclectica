package main

import (
  "os"
  "fmt"
  "time"

  "github.com/markelog/archive"

  "github.com/urfave/cli"
  "github.com/cavaliercoder/grab"

  "github.com/markelog/eclectica/variables"
  "github.com/markelog/eclectica/detect"
  "github.com/markelog/eclectica/directory"
  "github.com/markelog/eclectica/activation"
)

func exists(name string) bool {
  _, err := os.Stat(name)
  return !os.IsNotExist(err)
}


func main() {
  cli.AppHelpTemplate = `
Usage: e <name>, <name>@<version>

`
  if len(os.Args) == 1 {
    cli.NewApp().Run(os.Args)
  } else {

    dists, err := detect.Detect(os.Args[1])

    if err != nil {
      fmt.Println(err)
      os.Exit(1)
    }

    path := fmt.Sprintf("%s/%s/%s", variables.Home, dists["name"], dists["version"])

    if exists(path) {
      activation.Activate(path)
      os.Exit(0)
    }

    downloadPlace := download(dists["url"])

    fmt.Println("Extract archive")
    extractionPlace, err := directory.Create(dists["name"])

    if err != nil {
      fmt.Println(err)
      os.Exit(1)
    }

    err = archive.Extract(downloadPlace, extractionPlace)

    if err != nil {
      fmt.Println(err)
      os.Exit(1)
    }

    downloadPath := fmt.Sprintf("%s/%s", downloadPlace, dists["filename"])
    extractionPath := fmt.Sprintf("%s/%s", extractionPlace, dists["version"])
    os.Rename(downloadPath, extractionPath)

    fmt.Println("one")
    activation.Activate(path)
  }
}

func download(url string) string {

  // Start file download
  fmt.Printf("Downloading %s...\n", url)

  respch, err := grab.GetAsync(os.TempDir(), url)

  if err != nil {
    fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", url, err)
    os.Exit(1)
  }

  // Block until HTTP/1.1 GET response is received
  fmt.Printf("Initializing download...\n")

  resp := <-respch

  // Print progress until transfer is complete
  for !resp.IsComplete() {
    fmt.Printf("\033[1AProgress %d / %d bytes (%d%%)\033[K\n", resp.BytesTransferred(), resp.Size, int(100*resp.Progress()))
    time.Sleep(200 * time.Millisecond)
  }

  // Clear progress line
  fmt.Printf("\033[1A\033[K")

  // Check for errors
  if resp.Error != nil {
    fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", url, resp.Error)
    os.Exit(1)
  }

  fmt.Printf("Downloaded\n")

  return resp.Filename
}
