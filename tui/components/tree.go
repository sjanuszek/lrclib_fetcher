package components

import (
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func addChildrenToNode(target *tview.TreeNode, path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, file := range files {
		node := tview.NewTreeNode(file.Name()).SetReference(filepath.Join(path, file.Name()))

		if file.IsDir() {
			node.SetColor(tcell.ColorGreen)
		}

		target.AddChild(node)
	}
}

func setRoot(tree *tview.TreeView, rootDir string) {
	root := tview.NewTreeNode(rootDir).SetColor(tcell.ColorRed)
	tree.SetRoot(root).SetCurrentNode(root)

	addChildrenToNode(root, rootDir)
}

func MakeTreeBrowser(onSelect func(string), onCancel func()) *tview.TreeView {
	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	tree := tview.NewTreeView()

	setRoot(tree, rootDir)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			rootDir = filepath.Dir(rootDir)
			setRoot(tree, rootDir)
			return 
		}

		path := reference.(string)
		children := node.GetChildren()

		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
		    onSelect(path)
		    return
		}

		if len(children) == 0 {
			path := reference.(string)
			addChildrenToNode(node, path)
		} else {
			node.SetExpanded(!node.IsExpanded())
		}
	})

	tree.SetDoneFunc(func(key tcell.Key) {
		if node := tree.GetCurrentNode(); node != nil {
			switch key {
				case tcell.KeyTab:
					reference := node.GetReference()
					if reference == nil {
						return
					}

					rootDir = reference.(string)
					setRoot(tree, rootDir)
				case tcell.KeyBacktab:
					rootDir = filepath.Dir(rootDir)
					setRoot(tree, rootDir)
				case tcell.KeyESC:
					onCancel()
			}
		}
	})
	tree.SetBorder(true).
		SetTitle("File browser").
		SetTitleAlign(tview.AlignLeft)
		
	return tree
}
