package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/2found/2build/internal/ticket"
)

// A project approval covers the reviewed artifacts, not whatever the same
// paths happen to contain after a resume or a scope change.
func projectApprovalRevision(st *ticket.Store, doc ticket.Doc) (string, error) {
	h := sha256.New()
	for _, name := range []string{"requirement", "plan", "design", "prototype", "manifest"} {
		path := doc.Get("pointers." + name)
		explicit := path != ""
		if !explicit {
			path = name + ".md"
			if name == "prototype" {
				path = "prototype.html"
			}
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(st.Home(), path)
		}
		body, err := os.ReadFile(path)
		if err != nil && !(os.IsNotExist(err) && !explicit && name != "requirement" && name != "plan") {
			return "", fmt.Errorf("project-plan artifact %s: %w", name, err)
		}
		fmt.Fprintf(h, "%s\x00%s\x00%t\x00%x\n", name, path, err == nil, sha256.Sum256(body))
	}
	// Preserve legacy fingerprints until a structured contract is introduced.
	// Its durable pointer makes removal an error, never a return to legacy.
	path := projectContractPath(st)
	if body, err := os.ReadFile(path); err == nil {
		fmt.Fprintf(h, "project_contract\x00%x\n", sha256.Sum256(body))
	} else if !os.IsNotExist(err) || doc.Get("pointers.project_contract") != "" {
		return "", fmt.Errorf("project contract: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// approvalArtifactRevision fingerprints what a project-plan or release
// approval is about; a release go covers one exact release request.
func approvalArtifactRevision(st *ticket.Store, doc ticket.Doc, kind string) (string, error) {
	if kind == "release" {
		path := projectReleaseRequestPath(st)
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("release request: %w", err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(body)), nil
	}
	return projectApprovalRevision(st, doc)
}

func projectApprovalCurrent(st *ticket.Store, doc ticket.Doc) bool {
	kind := doc.Get("approval.kind")
	if kind != "project-plan" && kind != "release" {
		return true
	}
	revision, err := approvalArtifactRevision(st, doc, kind)
	return err == nil && revision == doc.Get("approval.artifact_revision")
}
