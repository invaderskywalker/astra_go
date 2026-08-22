package workspace

import (
	"os"
	"path/filepath"
)

func NewWorkspace(root string) (*Workspace, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	return &Workspace{
		Root: root,
	}, nil
}
