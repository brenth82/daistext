package main

import (
	"fmt"
	"unsafe"

	"github.com/brenth82/daistext/internal/daisy"
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
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
	items   map[win.HTREEITEM]*sectionItem
}

func (m *sectionsModel) RootCount() int             { return len(m.roots) }
func (m *sectionsModel) RootAt(i int) walk.TreeItem { return m.roots[i] }

func newSectionsModel(book *daisy.Book) *sectionsModel {
	m := &sectionsModel{
		book:    book,
		handles: make([]win.HTREEITEM, len(book.Sections)),
		items:   make(map[win.HTREEITEM]*sectionItem, len(book.Sections)),
	}
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

func enableSectionCheckboxes(tree *walk.TreeView, model func() *sectionsModel) error {
	handle := tree.Handle()
	comctl := windows.NewLazySystemDLL("comctl32.dll")
	setSubclass := comctl.NewProc("SetWindowSubclass")
	removeSubclass := comctl.NewProc("RemoveWindowSubclass")
	defSubclassProc := comctl.NewProc("DefSubclassProc")
	// Keep native checkbox states for display and accessibility, and handle
	// their mouse and keyboard input so a toggle updates the whole branch.
	callback := windows.NewCallback(func(hwnd win.HWND, msg uint32, wParam, lParam, _, _ uintptr) uintptr {
		if m := model(); m != nil {
			switch msg {
			case win.WM_KEYDOWN:
				if item, ok := tree.CurrentItem().(*sectionItem); wParam == win.VK_SPACE && ok {
					m.toggleBranch(tree, item)
					return 0
				}
			case win.WM_LBUTTONDOWN, win.WM_LBUTTONDBLCLK:
				hit := win.TVHITTESTINFO{Pt: win.POINT{
					X: int32(int16(lParam)),
					Y: int32(int16(lParam >> 16)),
				}}
				tree.SendMessage(win.TVM_HITTEST, 0, uintptr(unsafe.Pointer(&hit)))
				if item := m.items[hit.HItem]; item != nil && hit.Flags&win.TVHT_ONITEMSTATEICON != 0 {
					if msg == win.WM_LBUTTONDOWN {
						tree.SetFocus()
						m.toggleBranch(tree, item)
					}
					return 0
				}
			}
		}
		result, _, _ := defSubclassProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	})
	if ok, _, _ := setSubclass.Call(uintptr(handle), callback, 0, 0); ok == 0 {
		return fmt.Errorf("could not initialize section checkbox handling")
	}
	win.SetWindowLong(handle, win.GWL_STYLE, win.GetWindowLong(handle, win.GWL_STYLE)|win.TVS_CHECKBOXES)
	tree.Disposing().Attach(func() {
		removeSubclass.Call(uintptr(handle), callback, 0)
		const stateImageList = 2 // TVSIL_STATE
		if images := tree.SendMessage(win.TVM_GETIMAGELIST, stateImageList, 0); images != 0 {
			tree.SendMessage(win.TVM_SETIMAGELIST, stateImageList, 0)
			win.ImageList_Destroy(win.HIMAGELIST(images))
		}
	})
	return nil
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
			m.items[win.HTREEITEM(handle)] = item
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
	state := tree.SendMessage(win.TVM_GETITEMSTATE, uintptr(m.handles[i]), win.TVIS_STATEIMAGEMASK)
	return state&win.TVIS_STATEIMAGEMASK == checkedState
}

func (m *sectionsModel) selectedIDs(tree *walk.TreeView) []int {
	var ids []int
	for i := range m.handles {
		if m.checked(tree, i) {
			ids = append(ids, i)
		}
	}
	return ids
}

func (m *sectionsModel) setAllChecked(tree *walk.TreeView, checked bool) {
	for i := range m.handles {
		m.setChecked(tree, i, checked)
	}
}

func (m *sectionsModel) toggleBranch(tree *walk.TreeView, item *sectionItem) {
	checked := !m.checked(tree, item.index)
	m.setChecked(tree, item.index, checked)
	depth := m.book.Sections[item.index].Depth
	for i := item.index + 1; i < len(m.handles) && m.book.Sections[i].Depth > depth; i++ {
		m.setChecked(tree, i, checked)
	}
}

func (m *sectionsModel) setChecked(tree *walk.TreeView, i int, checked bool) {
	item := win.TVITEM{Mask: win.TVIF_STATE, HItem: m.handles[i], StateMask: win.TVIS_STATEIMAGEMASK, State: uncheckedState}
	if checked {
		item.State = checkedState
	}
	tree.SendMessage(win.TVM_SETITEM, 0, uintptr(unsafe.Pointer(&item)))
}
