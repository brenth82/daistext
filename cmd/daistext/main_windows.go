//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brenth82/daistext/internal/daisy"
	"github.com/brenth82/daistext/internal/export"
	"github.com/brenth82/daistext/internal/settings"
	"github.com/brenth82/daistext/internal/update"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"
)

var version = "0.1.0"

type app struct {
	mw                                         *walk.MainWindow
	tree                                       *walk.TreeView
	reporterCleanup                            *walk.CheckBox
	preview                                    *walk.TextEdit
	bookPanel                                  *walk.Composite
	welcome                                    *walk.Label
	status                                     *walk.Label
	format                                     *walk.ComboBox
	open, openZIP, convert, check, preferences *walk.PushButton
	model                                      *sectionsModel
	settings                                   settings.Settings
	ctx                                        context.Context
	cancel                                     context.CancelFunc
	busy                                       bool
	workers                                    sync.WaitGroup
}

var formats = []string{"txt", "md", "html", "pdf", "brf"}

func main() {
	// Inno Setup checks this mutex before installation and uninstallation.
	mutexName, _ := windows.UTF16PtrFromString(`Local\DaisText.Running`)
	mutex, e := windows.CreateMutex(nil, false, mutexName)
	if mutex == 0 {
		walk.MsgBox(nil, "DaisText", fmt.Sprintf("Cannot initialize application: %v", e), walk.MsgBoxIconError)
		return
	}
	defer windows.CloseHandle(mutex)
	a := &app{}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()
	s, loadErr := settings.Load()
	a.settings = s
	err := (MainWindow{AssignTo: &a.mw, Title: "DaisText", MinSize: Size{Width: 720, Height: 200}, Size: Size{Width: 920, Height: 250}, Layout: VBox{}, Children: []Widget{
		Composite{Layout: HBox{}, Children: []Widget{
			PushButton{AssignTo: &a.open, Text: "Open DAISY &folder…", Visible: s.ShowFolderImport, OnClicked: a.openFolder},
			PushButton{AssignTo: &a.openZIP, Text: "Open &ZIP…", Visible: s.ShowZIPImport, OnClicked: a.openBook},
			PushButton{AssignTo: &a.preferences, Text: "&Settings…", OnClicked: a.editSettings},
			PushButton{AssignTo: &a.check, Text: "Check for &updates…", OnClicked: func() { a.checkUpdates(true) }},
			PushButton{Text: "Abou&t…", OnClicked: a.showAbout},
			HSpacer{},
		}},
		Label{AssignTo: &a.welcome, Text: "Open a DAISY folder or ZIP to select sections and convert them.\r\nFor newspaper emails with separate attachments, save the OPF, NCX, SMIL and XML files together in one folder.\r\nYou can hide either Open option in Settings. Converted files default to your Downloads folder."},
		Composite{AssignTo: &a.bookPanel, Visible: false, Layout: VBox{}, StretchFactor: 1, Children: []Widget{
			Label{Text: "&Sections (Right and Left expand or collapse, Space toggles inclusion)"},
			TreeView{AssignTo: &a.tree, Accessibility: Accessibility{Name: "Sections in reading order", Description: "Right and Left expand or collapse branches. Space toggles inclusion."}, OnCurrentItemChanged: a.showPreview, StretchFactor: 3},
			Composite{Layout: HBox{}, Children: []Widget{PushButton{Text: "Select &all", OnClicked: func() { a.selectAll(true) }}, PushButton{Text: "Select &none", OnClicked: func() { a.selectAll(false) }}, PushButton{Text: "Toggle selected &branch", OnClicked: a.toggleBranch}, HSpacer{}}},
			CheckBox{AssignTo: &a.reporterCleanup, Text: "Skip &reporter contact details and bios", Visible: false, OnCheckedChanged: a.showPreview},
			Label{Text: "&Preview of highlighted section:"}, TextEdit{AssignTo: &a.preview, Accessibility: Accessibility{Name: "Section preview"}, ReadOnly: true, VScroll: true, StretchFactor: 2},
			Composite{Layout: HBox{}, Children: []Widget{Label{Text: "Output &format:"}, ComboBox{AssignTo: &a.format, Accessibility: Accessibility{Name: "Output format"}, Model: formats, CurrentIndex: 0}, PushButton{AssignTo: &a.convert, Text: "&Convert selected…", Enabled: false, OnClicked: a.convertBook}, HSpacer{}}},
		}},
		Label{AssignTo: &a.status, Text: "Open a text-bearing DAISY folder or ZIP to begin. Conversion stays on this computer."},
	}}).Create()
	if err != nil {
		walk.MsgBox(nil, "DaisText", err.Error(), walk.MsgBoxIconError)
		return
	}
	enableSectionCheckboxes(a.tree)
	for i, f := range formats {
		if f == s.Format {
			a.format.SetCurrentIndex(i)
		}
	}
	a.mw.Closing().Attach(func(_ *bool, _ walk.CloseReason) { a.cancel() })
	if loadErr != nil {
		a.failure(loadErr)
	}
	if len(os.Args) > 1 {
		a.load(os.Args[1])
	}
	if s.AutoUpdate && update.PublicKey != "" {
		a.startWorker(func() {
			select {
			case <-time.After(2 * time.Second):
				a.mw.Synchronize(func() {
					if a.ctx.Err() == nil && !a.busy {
						a.checkUpdates(false)
					}
				})
			case <-a.ctx.Done():
			}
		})
	}
	a.mw.Run()
	a.cancel()
	// Let canceled conversions remove their temporary source/output files.
	a.workers.Wait()
}

func (a *app) startWorker(work func()) {
	a.workers.Add(1)
	go func() { defer a.workers.Done(); work() }()
}
func (a *app) failure(e error) {
	walk.MsgBox(a.mw, "DaisText", e.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
}

func (a *app) showAbout() {
	message := "DaisText version " + version + "\r\n\r\n" +
		"Convert selected sections of DAISY books and newspapers.\r\n\r\n" +
		"Copyright © 2026 Brent Harding\r\n" +
		"Licensed under the MIT License. Bundled tools have their own licenses.\r\n\r\n" +
		"https://github.com/brenth82/daistext"
	walk.MsgBox(a.mw, "About DaisText", message, walk.MsgBoxOK|walk.MsgBoxIconInformation)
}
func (a *app) setBusy(v bool, message string) {
	a.busy = v
	a.open.SetEnabled(!v)
	a.openZIP.SetEnabled(!v)
	a.preferences.SetEnabled(!v)
	a.check.SetEnabled(!v)
	a.convert.SetEnabled(!v && a.model != nil)
	a.tree.SetEnabled(!v)
	a.reporterCleanup.SetEnabled(!v)
	a.status.SetText(message)
}
func (a *app) background(message string, work func() error, done func()) {
	a.setBusy(true, message)
	a.startWorker(func() {
		e := work()
		if a.ctx.Err() != nil {
			return
		}
		a.mw.Synchronize(func() {
			if a.ctx.Err() != nil {
				return
			}
			a.setBusy(false, "Ready.")
			if e != nil {
				a.failure(e)
				return
			}
			if done != nil {
				done()
			}
		})
	})
}
func (a *app) openFolder() {
	d := walk.FileDialog{Title: "Choose the folder containing the DAISY OPF or ncc.html"}
	if ok, e := d.ShowBrowseFolder(a.mw); e != nil {
		a.failure(e)
	} else if ok {
		a.load(d.FilePath)
	}
}
func (a *app) openBook() {
	d := walk.FileDialog{Title: "Open DAISY book", Filter: "DAISY books (*.zip;*.daisy)|*.zip;*.daisy|All files (*.*)|*.*"}
	ok, e := d.ShowOpen(a.mw)
	if e != nil {
		a.failure(e)
	} else if ok {
		a.load(d.FilePath)
	}
}
func (a *app) load(p string) {
	var b *daisy.Book
	a.background("Reading DAISY navigation and text…", func() error { var e error; b, e = daisy.Open(p); return e }, func() {
		firstLoad := a.model == nil
		m := newSectionsModel(b)
		a.model = nil
		if err := m.attach(a.tree); err != nil {
			a.bookPanel.SetVisible(false)
			a.welcome.SetVisible(true)
			a.convert.SetEnabled(false)
			a.failure(err)
			return
		}
		a.model = m
		a.reporterCleanup.SetChecked(false)
		a.reporterCleanup.SetVisible(b.Newspaper)
		a.welcome.SetVisible(false)
		a.bookPanel.SetVisible(true)
		if firstLoad {
			a.mw.SetMinMaxSize(walk.Size{Width: 720, Height: 540}, walk.Size{})
			a.mw.SetSize(walk.Size{Width: 1000, Height: 760})
		}
		a.convert.SetEnabled(true)
		a.mw.SetTitle(b.Title + " — DaisText")
		a.status.SetText(fmt.Sprintf("%d sections loaded. Uncheck sections to omit them. Parent checkboxes control only their own text; use Toggle selected branch for descendants.", len(b.Sections)))
		if len(b.Sections) > 0 {
			a.tree.SetCurrentItem(m.roots[0])
			a.showPreview()
		}
	})
}
func (a *app) showPreview() {
	if a.model == nil {
		return
	}
	item, ok := a.tree.CurrentItem().(*sectionItem)
	if ok {
		text := export.Text(a.model.book.SectionBlocks(item.index, a.reporterCleanup.Checked()))
		if len(text) > 100000 {
			text = text[:100000] + "\r\n[Preview shortened]"
		}
		a.preview.SetText(text)
	}
}
func (a *app) selectAll(v bool) {
	if a.model == nil {
		return
	}
	for i := range a.model.handles {
		a.model.setChecked(a.tree, i, v)
	}
}
func (a *app) toggleBranch() {
	if a.model == nil {
		return
	}
	item, ok := a.tree.CurrentItem().(*sectionItem)
	if !ok {
		return
	}
	i := item.index
	depth := a.model.book.Sections[i].Depth
	v := !a.model.checked(a.tree, i)
	a.model.setChecked(a.tree, i, v)
	for j := i + 1; j < len(a.model.handles) && a.model.book.Sections[j].Depth > depth; j++ {
		a.model.setChecked(a.tree, j, v)
	}
}
func (a *app) convertBook() {
	if a.model == nil {
		return
	}
	var ids []int
	for i := range a.model.handles {
		if a.model.checked(a.tree, i) {
			ids = append(ids, i)
		}
	}
	blocks, e := a.model.book.SelectedForExport(ids, a.reporterCleanup.Checked())
	if e != nil {
		a.failure(e)
		return
	}
	f := formats[a.format.CurrentIndex()]
	d := walk.FileDialog{Title: "Save converted book", InitialDirPath: a.settings.OutputDirectory, FilePath: filepath.Join(a.settings.OutputDirectory, "converted."+f), Filter: strings.ToUpper(f) + " file (*." + f + ")|*." + f, Flags: 2}
	ok, e := d.ShowSave(a.mw)
	if e != nil {
		a.failure(e)
		return
	}
	if !ok {
		return
	}
	b := a.model.book
	o := a.settings.Tools
	a.background("Converting selected sections…", func() error { return export.Write(a.ctx, d.FilePath, f, b.Title, b.Language, blocks, o) }, func() {
		a.settings.Format = f
		if e := settings.Save(a.settings); e != nil {
			a.failure(e)
		}
		a.status.SetText("Saved " + d.FilePath)
		walk.MsgBox(a.mw, "Conversion complete", "Saved "+d.FilePath, walk.MsgBoxOK|walk.MsgBoxIconInformation)
	})
}
func (a *app) editSettings() {
	var dlg *walk.Dialog
	var auto, showZIP, showFolder *walk.CheckBox
	var typst, louis, table, folder *walk.LineEdit
	var cells, lines *walk.NumberEdit
	var save *walk.PushButton
	s := a.settings
	err := (Dialog{AssignTo: &dlg, Title: "DaisText settings", Layout: VBox{}, MinSize: Size{Width: 650, Height: 400}, DefaultButton: &save, Children: []Widget{
		CheckBox{AssignTo: &auto, Text: "Automatically check for updates at &startup (ask before installing)", Checked: s.AutoUpdate},
		CheckBox{AssignTo: &showZIP, Text: "Show Open &ZIP", Checked: s.ShowZIPImport},
		CheckBox{AssignTo: &showFolder, Text: "Show Open DAISY &folder", Checked: s.ShowFolderImport},
		Label{Text: "&Default output folder (blank: Downloads):"}, Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &folder, Accessibility: Accessibility{Name: "Default output folder"}, Text: s.OutputDirectory}, PushButton{Text: "Browse…", OnClicked: func() { a.pickFolder(dlg, folder) }}, PushButton{Text: "Use Downloads", OnClicked: func() { folder.SetText(settings.DownloadsDirectory()) }}}},
		Label{Text: "&Typst executable (blank: bundled tool, then PATH):"}, Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &typst, Accessibility: Accessibility{Name: "Typst executable"}, Text: s.Tools.TypstPath}, PushButton{Text: "Browse…", OnClicked: func() { a.pickTool(dlg, typst) }}}},
		Label{Text: "&Liblouis lou_translate executable (blank: bundled tool, then PATH):"}, Composite{Layout: HBox{}, Children: []Widget{LineEdit{AssignTo: &louis, Accessibility: Accessibility{Name: "Liblouis executable"}, Text: s.Tools.LouisPath}, PushButton{Text: "Browse…", OnClicked: func() { a.pickTool(dlg, louis) }}}},
		Label{Text: "&Braille table chain (blank: bundled UEB contracted English):"}, LineEdit{AssignTo: &table, Accessibility: Accessibility{Name: "Braille table chain"}, Text: s.Tools.BrailleTable},
		Composite{Layout: Grid{Columns: 4}, Children: []Widget{Label{Text: "&Cells per line:"}, NumberEdit{AssignTo: &cells, Accessibility: Accessibility{Name: "Cells per line"}, Value: float64(s.Tools.CellsPerLine), MinValue: 10, MaxValue: 120, Decimals: 0}, Label{Text: "Lines per &page:"}, NumberEdit{AssignTo: &lines, Accessibility: Accessibility{Name: "Lines per page"}, Value: float64(s.Tools.LinesPerPage), MinValue: 5, MaxValue: 100, Decimals: 0}}},
		Label{Text: "PDF requires Typst 0.14+. BRF requires Liblouis and tables; translation runs locally.\r\nManual update checks remain available when automatic checks are off."},
		Composite{Layout: HBox{}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &save, Text: "&Save", OnClicked: func() {
			s.AutoUpdate = auto.Checked()
			s.ShowZIPImport = showZIP.Checked()
			s.ShowFolderImport = showFolder.Checked()
			s.OutputDirectory = strings.TrimSpace(folder.Text())
			if s.OutputDirectory == "" {
				s.OutputDirectory = settings.DownloadsDirectory()
			}
			p, e := filepath.Abs(s.OutputDirectory)
			if e != nil {
				a.failure(e)
				return
			}
			s.OutputDirectory = p
			if info, e := os.Stat(p); e != nil {
				a.failure(e)
				return
			} else if !info.IsDir() {
				a.failure(fmt.Errorf("choose an existing output folder"))
				return
			}
			s.Tools.TypstPath = strings.TrimSpace(typst.Text())
			s.Tools.LouisPath = strings.TrimSpace(louis.Text())
			s.Tools.BrailleTable = strings.TrimSpace(table.Text())
			s.Tools.CellsPerLine = int(cells.Value())
			s.Tools.LinesPerPage = int(lines.Value())
			if e := settings.Save(s); e != nil {
				a.failure(e)
				return
			}
			a.settings = s
			a.openZIP.SetVisible(s.ShowZIPImport)
			a.open.SetVisible(s.ShowFolderImport)
			dlg.Accept()
		}}, PushButton{Text: "Cancel", OnClicked: func() { dlg.Cancel() }}}},
	}}).Create(a.mw)
	if err != nil {
		a.failure(err)
		return
	}
	dlg.Run()
}
func (a *app) pickFolder(owner walk.Form, edit *walk.LineEdit) {
	d := walk.FileDialog{Title: "Choose the default output folder"}
	if ok, e := d.ShowBrowseFolder(owner); e != nil {
		a.failure(e)
	} else if ok {
		edit.SetText(d.FilePath)
	}
}
func (a *app) pickTool(owner walk.Form, edit *walk.LineEdit) {
	d := walk.FileDialog{Title: "Choose executable", Filter: "Executables (*.exe)|*.exe"}
	if ok, e := d.ShowOpen(owner); e != nil {
		a.failure(e)
	} else if ok {
		edit.SetText(d.FilePath)
	}
}
func (a *app) checkUpdates(manual bool) {
	if a.busy {
		return
	}
	var m *update.Manifest
	a.setBusy(true, "Checking for a signed update…")
	a.startWorker(func() {
		result, e := update.Check(a.ctx, version)
		m = result
		if a.ctx.Err() != nil {
			return
		}
		a.mw.Synchronize(func() {
			if a.ctx.Err() != nil {
				return
			}
			a.setBusy(false, "Ready.")
			if e != nil {
				if manual {
					a.failure(e)
				} else {
					a.status.SetText("Automatic update check failed. Use Check for updates for details.")
				}
				return
			}
			if m == nil {
				if manual {
					walk.MsgBox(a.mw, "DaisText", "You have the latest available release.", walk.MsgBoxOK)
				}
				return
			}
			if walk.MsgBox(a.mw, "Update available", "DaisText "+m.Version+" is available. Download and install the verified update now? DaisText will close and reopen after installation.", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
				return
			}
			var installer string
			a.background("Downloading and verifying the update…", func() error { var e error; installer, e = update.Download(a.ctx, *m); return e }, func() {
				cmd := exec.Command(installer, "/SP-", "/SILENT", "/NORESTART", "/CURRENTUSER", "/DAISTEXTUPDATE=1")
				if e := cmd.Start(); e != nil {
					a.failure(e)
					return
				}
				a.mw.Close()
			})
		})
	})
}
