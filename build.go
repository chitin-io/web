// Command build writes out the static HTML for the Chitin website.
//
// Usage:
//
//     go run build.go
//
// You will need a Go development environment and the necessary
// libraries installed.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/russross/blackfriday"
	"github.com/tdewolff/minify"
	mincss "github.com/tdewolff/minify/css"
	minhtml "github.com/tdewolff/minify/html"
	minjs "github.com/tdewolff/minify/js"
	minjson "github.com/tdewolff/minify/json"
	minsvg "github.com/tdewolff/minify/svg"
	minxml "github.com/tdewolff/minify/xml"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const outputDir = "output"

var builders = map[string]func(path string, info os.FileInfo) error{
	".md":  markdown,
	".dot": graphvizDot,
}

var layout = template.Must(template.ParseFiles("template.html"))

var minifier = minify.New()

func init() {
	minifier.AddFunc("text/css", mincss.Minify)
	minifier.AddFunc("text/html", minhtml.Minify)
	minifier.AddFunc("text/javascript", minjs.Minify)
	minifier.AddFunc("image/svg+xml", minsvg.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("[/+]json$"), minjson.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), minxml.Minify)
}

// get the text content of children of this node
func childText(node *html.Node) string {
	var s string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			s += child.Data
		case html.ElementNode:
			s += childText(child)
		}
	}
	return s
}

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := ioutil.TempFile(dir, ".tmp-")
	if err != nil {
		return err
	}
	defer func() {
		if tmp != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	tmp = nil
	return nil
}

func markdown(path string, info os.FileInfo) error {
	input, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	// extract the header out of the markdown, so we can control the
	// layout better; blackfriday would put the toc above the h1, and
	// include the singular h1 in the toc, causing stutter.
	idx := bytes.IndexByte(input, '\n')
	if idx == -1 {
		return errors.New("markdown has no content")
	}
	titleMD, input := input[:idx], input[idx+1:]

	htmlFlags := (0 |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_FRACTIONS |
		blackfriday.HTML_SMARTYPANTS_LATEX_DASHES |
		blackfriday.HTML_USE_XHTML |
		blackfriday.HTML_FOOTNOTE_RETURN_LINKS |
		0)
	// HtmlRenderer demands a title and a css path here, but we only
	// render a fragment so those are not used
	renderer := blackfriday.HtmlRenderer(htmlFlags, "", "")
	extensions := (0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_FOOTNOTES |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_AUTO_HEADER_IDS |
		0)
	titleHTML := blackfriday.Markdown(titleMD, renderer, extensions)
	contentHTML := blackfriday.Markdown(input, renderer, extensions)

	tocFlags := htmlFlags | blackfriday.HTML_TOC | blackfriday.HTML_OMIT_CONTENTS
	tocRenderer := blackfriday.HtmlRenderer(tocFlags, "", "")
	tocHTML := blackfriday.Markdown(input, tocRenderer, extensions)
	body := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}
	nodes, err := html.ParseFragment(bytes.NewReader(titleHTML), body)
	if err != nil {
		return fmt.Errorf("cannot parse generated html: %v", err)
	}
	if len(nodes) == 0 ||
		nodes[0].Type != html.ElementNode ||
		nodes[0].DataAtom != atom.H1 {
		return errors.New("markdown does not start with a header")
	}
	title := childText(nodes[0])

	var buf bytes.Buffer
	data := struct {
		Title   string
		H1      template.HTML
		TOC     template.HTML
		Content template.HTML
	}{
		Title:   title,
		H1:      template.HTML(titleHTML),
		TOC:     template.HTML(tocHTML),
		Content: template.HTML(contentHTML),
	}
	if err := layout.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template: %v", err)
	}

	min, err := minify.Bytes(minifier, "text/html", buf.Bytes())
	if err != nil {
		return fmt.Errorf("cannot minify html: %v", err)
	}

	dst := filepath.Join(outputDir, strings.TrimSuffix(path, ".md")+".html")
	if err := writeFile(dst, min); err != nil {
		return err
	}
	return nil
}

func graphvizDot(path string, info os.FileInfo) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = f
	buf, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error running dot: %v", err)
	}

	min, err := minify.Bytes(minifier, "image/svg+xml", buf)
	if err != nil {
		return fmt.Errorf("cannot minify svg: %v", err)
	}

	dst := filepath.Join(outputDir, strings.TrimSuffix(path, ".dot")+".svg")
	if err := writeFile(dst, min); err != nil {
		return err
	}
	return nil
}

func processFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if path == outputDir {
		// do not recurse into our output directory
		return filepath.SkipDir
	}

	if info.Name()[0] == '.' {
		// ignore hidden files, do not recurse into hidden dirs
		if info.IsDir() && info.Name() != "." {
			return filepath.SkipDir
		}
		return nil
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	if info.Name() == "README.md" {
		return nil
	}

	ext := filepath.Ext(info.Name())
	if fn, ok := builders[ext]; ok {
		log.Printf("source %v", path)
		if err := fn(path, info); err != nil {
			return fmt.Errorf("build failed: %v: %v", path, err)
		}
	}
	return nil
}

// environReplace updates the list of environment variables to add or
// replace the given variable.
func environReplace(env []string, name, val string) []string {
	r := env[:0]
	prefix := name + "="
	for _, s := range env {
		if strings.HasPrefix(s, prefix) {
			continue
		}
		r = append(r, s)
	}
	r = append(r, name+"="+val)
	return r
}

func gitDescribe() (string, error) {
	cmd := exec.Command("git", "describe", "--always", "--dirty")
	cmd.Stderr = os.Stderr
	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}
	desc, err := oneLine(buf)
	if err != nil {
		return "", fmt.Errorf("cannot parse git describe output: %v", err)
	}
	return desc, nil
}

func gitAddOutput(index string) error {
	cmd := exec.Command("git", "update-index", "--add", "-z", "--verbose", "--stdin")
	cmd.Env = environReplace(os.Environ(), "GIT_INDEX_FILE", index)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		if !cmd.ProcessState.Exited() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	fn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := io.WriteString(stdin, path+"\x00"); err != nil {
			return err
		}
		return nil
	}
	if err := filepath.Walk(outputDir, fn); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func oneLine(b []byte) (string, error) {
	if len(b) == 0 {
		return "", errors.New("must not be empty")
	}
	if b[len(b)-1] != '\n' {
		return "", fmt.Errorf("must end in newline: %q", b)
	}
	b = b[:len(b)-1]
	if idx := bytes.IndexByte(b, '\n'); idx >= 0 {
		return "", fmt.Errorf("must be a single line: %q", b)
	}
	return string(b), nil
}

func gitWriteTree(index string, prefix string) (string, error) {
	cmd := exec.Command("git", "write-tree", "--prefix="+prefix+"/")
	cmd.Env = environReplace(os.Environ(), "GIT_INDEX_FILE", index)
	cmd.Stderr = os.Stderr
	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}
	sha, err := oneLine(buf)
	if err != nil {
		return "", fmt.Errorf("cannot parse git write-tree output: %v", err)
	}
	return sha, nil
}

func gitCommit(tree string, message string) (string, error) {
	cmd := exec.Command("git", "commit-tree", "-m", message, tree)
	cmd.Stderr = os.Stderr

	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}
	sha, err := oneLine(buf)
	if err != nil {
		return "", fmt.Errorf("cannot parse git commit-tree output: %v", err)
	}
	return sha, nil
}

func gitUpdateRef(ref string, commit string) error {
	cmd := exec.Command("git", "update-ref", "-m", "Build", ref, commit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func writeToGit() error {
	index := ".git/index.build"
	// the index file must not actually exist before we call git
	if err := os.Remove(index); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		_ = os.Remove(index)
	}()

	desc, err := gitDescribe()
	if err != nil {
		return err
	}
	if err := gitAddOutput(index); err != nil {
		return err
	}
	tree, err := gitWriteTree(index, outputDir)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("Regenerated site from %s\n", desc)
	commit, err := gitCommit(tree, message)
	if err != nil {
		return err
	}
	if gitUpdateRef("refs/heads/autogenerated", commit); err != nil {
		return err
	}
	log.Printf("Prepared %s", commit)
	log.Printf("To see diff run\n\n  %s\n\n",
		"git diff autogenerated@{0} autogenerated")
	log.Printf("To push run\n\n  %s\n\n",
		"git push https://github.com/chitin-io/chitin-io.github.io +autogenerated:refs/heads/master")

	return nil
}

func run() error {
	if err := os.Mkdir(outputDir, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := filepath.Walk(".", processFile); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outputDir, ".nojekyll"), nil); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outputDir, "CNAME"), []byte("chitin.io\n")); err != nil {
		return err
	}

	if err := writeToGit(); err != nil {
		return err
	}
	return nil
}

var prog = filepath.Base(os.Args[0])

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s\n", prog)
	fmt.Fprintf(os.Stderr, "(the command takes no options)\n")
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(prog + ": ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}
