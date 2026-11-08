package main

import (
	"fmt"
	"unsafe"

	"github.com/brenth82/daistext/internal/daisy"
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

type sectionItem struct {
	index    int
	title    string
	parent   *sectionItem
	children []*sectionItem
}

func (s *sectionItem) Text() string { return s.title }
func (s *sectionItem) Parent() walk.TreeItem {
	if s.parent == nil {
		return nil
	}
	return s.parent
}
func (s *sectionItem) ChildCount() int             { return len(s.children) }
func (s *sectionItem) ChildAt(i int) walk.TreeItem { return s.children[i] }

type sectionsModel struct {
	walk.TreeModelBase
	book    *daisy.Book
	roots   []*sectionItem
	handles []win.HTREEITEM
}

func (m *sectionsModel) RootCount() int             { return len(m.roots) }
func (m *sectionsModel) RootAt(i int) walk.TreeItem { return m.roots[i] }

func newSectionsModel(book *daisy.Book) *sectionsModel {
	m := &sectionsModel{book: book, handles: make([]win.HTREEITEM, len(book.Sections))}
	var ancestors []*sectionItem
	for i, s := range book.Sections {
		for len(ancestors) > 0 && book.Sections[ancestors[len(ancestors)-1].index].Depth >= s.Depth {
			ancestors = ancestors[:len(ancestors)-1]
		}
		item := &sectionItem{index: i, title: s.Title}
		if len(ancestors) == 0 {
			m.roots = append(m.roots, item)
		} else {
			item.parent = ancestors[len(ancestors)-1]
			item.parent.children = append(item.parent.children, item)
		}
		ancestors = append(ancestors, item)
	}
	return m
}

func enableSectionCheckboxes(tree *walk.TreeView) {
	handle := tree.Handle()
	win.SetWindowLong(handle, win.GWL_STYLE, win.GetWindowLong(handle, win.GWL_STYLE)|win.TVS_CHECKBOXES)
	tree.Disposing().Attach(func() {
		const stateImageList = 2 // TVSIL_STATE
		if images := tree.SendMessage(win.TVM_GETIMAGELIST, stateImageList, 0); images != 0 {
			tree.SendMessage(win.TVM_SETIMAGELIST, stateImageList, 0)
			win.ImageList_Destroy(win.HIMAGELIST(images))
		}
	})
}

func (m *sectionsModel) attach(tree *walk.TreeView) error {
	if err := tree.SetModel(m); err != nil {
		return err
	}
	const (
		treeRoot  = 0 // TVGN_ROOT
		treeNext  = 1 // TVGN_NEXT
		treeChild = 4 // TVGN_CHILD
	)
	var bind func([]*sectionItem, uintptr) error
	bind = func(items []*sectionItem, handle uintptr) error {
		for _, item := range items {
			if handle == 0 {
				return fmt.Errorf("section tree is missing an item")
			}
			m.handles[item.index] = win.HTREEITEM(handle)
			m.setChecked(tree, item.index, true)
			if len(item.children) > 0 {
				child := tree.SendMessage(win.TVM_GETNEXTITEM, treeChild, handle)
				if err := bind(item.children, child); err != nil {
					return err
				}
			}
			handle = tree.SendMessage(win.TVM_GETNEXTITEM, treeNext, handle)
		}
		return nil
	}
	return bind(m.roots, tree.SendMessage(win.TVM_GETNEXTITEM, treeRoot, 0))
}

const (
	uncheckedState = 1 << 12
	checkedState   = 2 << 12
)

func (m *sectionsModel) checked(tree *walk.TreeView, i int) bool {
	return tree.SendMessage(win.TVM_GETITEMSTATE, uintptr(m.handles[i]), win.TVIS_STATEIMAGEMASK) == checkedState
}

func (m *sectionsModel) setChecked(tree *walk.TreeView, i int, checked bool) {
	item := win.TVITEM{Mask: win.TVIF_STATE, HItem: m.handles[i], StateMask: win.TVIS_STATEIMAGEMASK, State: uncheckedState}
	if checked {
		item.State = checkedState
	}
	tree.SendMessage(win.TVM_SETITEM, 0, uintptr(unsafe.Pointer(&item)))
}
