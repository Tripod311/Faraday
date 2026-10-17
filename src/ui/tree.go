package ui

import (
	"strings"
	"tripod311/faraday/vault"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

const vaultRootID = "##VAULT##"

type VaultTree struct {
	vaultName    string
	vault        *vault.Vault
	nodes        *map[string][]string
	SelectedNode string

	Widget         *widget.Tree
	OnNodeSelected func(id string)
	OnCreateNode   func(prefix string)
	OnDeleteNode   func(id string)
}

func CreateVaultTree(vaultName string, vault *vault.Vault) *VaultTree {
	nodes := make(map[string][]string)

	vt := &VaultTree{
		vaultName: vaultName,
		vault:     vault,
		nodes:     &nodes,
	}

	vt.Widget = widget.NewTreeWithStrings(nodes)

	vt.Widget.CreateNode = func(branch bool) fyne.CanvasObject {
		return newVaultTreeLabel()
	}

	vt.Widget.UpdateNode = func(
		id string,
		branch bool,
		obj fyne.CanvasObject,
	) {
		label := obj.(*vaultTreeLabel)

		if id == vaultRootID {
			label.SetText(vt.vaultName)
		} else {
			label.SetText(id[strings.LastIndex(id, "/")+1:])
		}

		label.onTapped = func() {
			if branch {
				vt.Widget.ToggleBranch(id)
				return
			}

			vt.Widget.Select(id)
		}

		label.onSecondary = func(event *fyne.PointEvent) {
			vt.showContextMenu(id, event.AbsolutePosition)
		}
	}

	vt.Widget.OnSelected = func(id string) {
		if vt.Widget.IsBranch(id) {
			vt.Widget.Unselect(id)
			return
		}

		path := vaultNodePath(id)
		vt.SelectedNode = path

		if vt.OnNodeSelected != nil {
			vt.OnNodeSelected(path)
		}
	}

	vt.Refresh()
	return vt
}

func (vt *VaultTree) Refresh() {
	nodes := *vt.nodes
	clear(nodes)

	nodes[""] = []string{vaultRootID}
	nodes[vaultRootID] = []string{}

	if vt.vault != nil {
		namespaces, _ := vt.vault.GetNested("")

		for _, ns := range namespaces {
			nsID := vaultRootID + "/" + ns

			nodes[vaultRootID] = append(nodes[vaultRootID], nsID)
			nodes[nsID] = []string{}

			entries, _ := vt.vault.GetNested(ns)

			for _, entry := range entries {
				path := entry
				if !strings.HasPrefix(path, ns+"/") {
					path = ns + "/" + entry
				}

				entryID := vaultRootID + "/" + path
				nodes[nsID] = append(nodes[nsID], entryID)
			}
		}
	}

	selectedExists := vt.SelectedNode == ""

	for _, children := range nodes {
		for _, id := range children {
			if vaultNodePath(id) == vt.SelectedNode {
				selectedExists = true
			}
		}
	}

	if !selectedExists {
		vt.Widget.UnselectAll()
		vt.SelectedNode = ""

		if vt.OnNodeSelected != nil {
			vt.OnNodeSelected("")
		}
	}

	vt.Widget.Refresh()
	vt.Widget.OpenBranch(vaultRootID)
}

func (vt *VaultTree) showContextMenu(
	id string,
	position fyne.Position,
) {
	path := vaultNodePath(id)
	isRoot := id == vaultRootID
	isNamespace := !isRoot && !strings.Contains(path, "/")

	prefix := ""
	if !isRoot {
		ns, _, _ := strings.Cut(path, "/")
		prefix = ns + "/"
	}

	items := make([]*fyne.MenuItem, 0, 3)

	if vt.OnCreateNode != nil {
		items = append(items,
			fyne.NewMenuItem("Add entry", func() {
				vt.OnCreateNode(prefix)
			}),
		)
	}

	if !isRoot && !isNamespace && vt.OnDeleteNode != nil {
		items = append(items,
			fyne.NewMenuItem("Delete", func() {
				vt.OnDeleteNode(path)
			}),
		)
	}

	if len(items) == 0 {
		return
	}

	canvas := fyne.CurrentApp().Driver().CanvasForObject(vt.Widget)
	if canvas == nil {
		return
	}

	widget.ShowPopUpMenuAtPosition(
		fyne.NewMenu("", items...),
		canvas,
		position,
	)
}

func vaultNodePath(id string) string {
	if id == vaultRootID {
		return ""
	}

	return strings.TrimPrefix(id, vaultRootID+"/")
}

type vaultTreeLabel struct {
	widget.Label
	onTapped    func()
	onSecondary func(*fyne.PointEvent)
}

func newVaultTreeLabel() *vaultTreeLabel {
	label := &vaultTreeLabel{}
	label.ExtendBaseWidget(label)
	return label
}

func (l *vaultTreeLabel) Tapped(_ *fyne.PointEvent) {
	if l.onTapped != nil {
		l.onTapped()
	}
}

func (l *vaultTreeLabel) TappedSecondary(event *fyne.PointEvent) {
	if l.onSecondary != nil {
		l.onSecondary(event)
	}
}
