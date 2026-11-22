//go:build daistext_gui_test

package main

import (
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/brenth82/daistext/internal/daisy"
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

func TestSectionCheckboxTogglesBranch(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	form, err := walk.NewMainWindow()
	if err != nil {
		t.Fatal(err)
	}
	defer form.Dispose()
	tree, err := walk.NewTreeView(form)
	if err != nil {
		t.Fatal(err)
	}
	form.SetSize(walk.Size{Width: 600, Height: 450})
	tree.SetBounds(walk.Rectangle{Width: 500, Height: 350})

	var model *sectionsModel
	if err := enableSectionCheckboxes(tree, func() *sectionsModel { return model }); err != nil {
		t.Fatal(err)
	}
	book := &daisy.Book{Sections: []daisy.Section{
		{Title: "Obituaries"},
		{Title: "Death notices", Depth: 1},
		{Title: "Obituary article", Depth: 2},
		{Title: "Local news"},
	}}
	next := newSectionsModel(book)
	if err := next.attach(tree); err != nil {
		t.Fatal(err)
	}
	model = next
	parent := model.roots[0]
	article := parent.children[0].children[0]

	assertSelected := func(want ...int) {
		t.Helper()
		if got := model.selectedIDs(tree); !slices.Equal(got, want) {
			t.Fatalf("selected sections = %v, want %v", got, want)
		}
	}
	pressSpace := func(item *sectionItem) {
		t.Helper()
		if err := tree.SetCurrentItem(item); err != nil {
			t.Fatal(err)
		}
		tree.SendMessage(win.WM_KEYDOWN, win.VK_SPACE, 0)
		tree.SendMessage(win.WM_CHAR, win.VK_SPACE, 0)
		tree.SendMessage(win.WM_KEYUP, win.VK_SPACE, 0)
	}
	assertSelected(0, 1, 2, 3)
	pressSpace(parent)
	assertSelected(3)
	pressSpace(article)
	assertSelected(2, 3)

	var rect win.RECT
	*(*win.HTREEITEM)(unsafe.Pointer(&rect)) = model.handles[parent.index]
	if tree.SendMessage(win.TVM_GETITEMRECT, 1, uintptr(unsafe.Pointer(&rect))) == 0 {
		t.Fatal("could not locate the parent item")
	}
	for x := rect.Left - 1; x >= 0; x-- {
		hit := win.TVHITTESTINFO{Pt: win.POINT{X: x, Y: (rect.Top + rect.Bottom) / 2}}
		tree.SendMessage(win.TVM_HITTEST, 0, uintptr(unsafe.Pointer(&hit)))
		if hit.HItem == model.handles[parent.index] && hit.Flags&win.TVHT_ONITEMSTATEICON != 0 {
			coords := uintptr(hit.Pt.Y)<<16 | uintptr(hit.Pt.X)
			tree.SendMessage(win.WM_LBUTTONDOWN, 1, coords)
			tree.SendMessage(win.WM_LBUTTONUP, 0, coords)
			assertSelected(0, 1, 2, 3)
			return
		}
	}
	t.Fatal("could not locate the parent checkbox")
}
