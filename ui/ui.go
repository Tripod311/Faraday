package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tripod311/faraday/vault"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type UI struct {
	vaultPath  string
	vaultKey   []byte
	vault      *vault.Vault
	app        fyne.App
	MainWindow fyne.Window
	content    *container.Split
	tree       *VaultTree
	editor     *widget.Entry
	skipChange bool
}

func CreateUI() *UI {
	a := app.NewWithID("Faraday")
	w := a.NewWindow("Faraday")

	res := UI{
		app:        a,
		MainWindow: w,
	}

	toolbar := container.NewHBox(
		widget.NewButtonWithIcon("Create", theme.StorageIcon(), res.onCreate),
		widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), res.onOpen),
		widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), res.onSave),
		widget.NewButton("Save as", res.onSaveAs),
		layout.NewSpacer(),
		widget.NewButtonWithIcon("Help", theme.HelpIcon(), res.onHelp),
	)

	content := container.NewHSplit(container.NewStack(), container.NewStack())
	res.content = content

	w.SetContent(container.NewBorder(
		toolbar,
		nil,
		nil,
		nil,
		content,
	))

	return &res
}

func (ui *UI) onCreate() {
	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, ui.MainWindow)
			return
		}
		if folder == nil {
			return
		}

		directory := folder.Path()

		vaultName := widget.NewEntry()
		vaultName.SetPlaceHolder("Vault name")

		password := widget.NewPasswordEntry()
		password.SetPlaceHolder("Vault password")

		d := dialog.NewForm(
			"Create vault",
			"Create",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Name", vaultName),
				widget.NewFormItem("Password", password),
			},
			func(ok bool) {
				if !ok {
					password.SetText("")
					return
				}

				fullPath := filepath.Join(directory, vaultName.Text)
				secret := []byte(password.Text)
				password.SetText("")

				loading := dialog.NewCustomWithoutButtons(
					"Creating vault",
					container.NewVBox(
						widget.NewLabel("Please wait…"),
						widget.NewProgressBarInfinite(),
					),
					ui.MainWindow,
				)
				loading.Show()

				go func() {
					var v *vault.Vault
					err := vault.CreateVault(fullPath, secret)
					if err == nil {
						v, err = ui.loadVault(fullPath, secret)
					}

					fyne.Do(func() {
						loading.Hide()

						if err != nil {
							dialog.ShowError(err, ui.MainWindow)
							return
						} else {
							ui.setVault(fullPath, secret, v)
						}
					})
				}()
			},
			ui.MainWindow,
		)

		d.Resize(fyne.NewSize(400, 160))
		d.Show()
		ui.MainWindow.Canvas().Focus(vaultName)
	}, ui.MainWindow)
}

func (ui *UI) onOpen() {
	dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ui.MainWindow)
			return
		}
		if file == nil {
			return
		}

		filename := file.URI().Path()
		file.Close()

		password := widget.NewPasswordEntry()

		d := dialog.NewForm(
			"Open vault",
			"Open",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Password", password),
			},
			func(ok bool) {
				if !ok {
					password.SetText("")
					return
				}

				secret := []byte(password.Text)
				password.SetText("")

				loading := dialog.NewCustomWithoutButtons(
					"Creating vault",
					container.NewVBox(
						widget.NewLabel("Please wait…"),
						widget.NewProgressBarInfinite(),
					),
					ui.MainWindow,
				)
				loading.Show()

				go func() {
					vault, err := ui.loadVault(filename, secret)

					fyne.Do(func() {
						loading.Hide()

						if err != nil {
							dialog.ShowError(err, ui.MainWindow)
							return
						} else {
							ui.setVault(filename, secret, vault)
						}
					})
				}()
			},
			ui.MainWindow,
		)

		d.Resize(fyne.NewSize(400, 160))
		d.Show()
		ui.MainWindow.Canvas().Focus(password)
	}, ui.MainWindow)
}

func (ui *UI) onSave() {
	newPassword := widget.NewPasswordEntry()
	newPassword.PlaceHolder = "Leave empty to preserve"

	d := dialog.NewForm(
		"Change password",
		"Submit",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("New password", newPassword),
		},
		func(ok bool) {
			if !ok {
				return
			}

			loading := dialog.NewCustomWithoutButtons(
				"Saving vault",
				container.NewVBox(
					widget.NewLabel("Please wait…"),
					widget.NewProgressBarInfinite(),
				),
				ui.MainWindow,
			)
			loading.Show()

			go func() {
				var pwdReplace [][]byte

				if len(newPassword.Text) > 0 {
					pwdReplace = append(pwdReplace, []byte(newPassword.Text))
				}

				serialized, err := ui.vault.Serialize()
				if err == nil {
					err = vault.WriteVault(ui.vaultPath, ui.vaultKey, serialized, pwdReplace...)
				}

				fyne.Do(func() {
					loading.Hide()

					if err != nil {
						dialog.ShowError(err, ui.MainWindow)
						return
					} else {
						if len(pwdReplace) > 0 {
							ui.vaultKey = pwdReplace[0]
						}
						dialog.ShowInformation("Success", "Vault saved", ui.MainWindow)
					}
				})
			}()
		},
		ui.MainWindow,
	)

	d.Resize(fyne.NewSize(400, 160))
	d.Show()
}

func (ui *UI) onSaveAs() {
	if ui.vault == nil {
		return
	}

	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, ui.MainWindow)
			return
		}
		if folder == nil {
			return
		}

		name := widget.NewEntry()
		name.SetText(filepath.Base(ui.vaultPath))

		nameDialog := dialog.NewForm(
			"Save vault as",
			"Continue",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Filename", name),
			},
			func(ok bool) {
				if !ok {
					return
				}

				if name.Text == "" ||
					name.Text == "." ||
					name.Text == ".." ||
					strings.ContainsAny(name.Text, `/\`) {
					dialog.ShowError(
						errors.New("invalid filename"),
						ui.MainWindow,
					)
					return
				}

				filename := filepath.Join(folder.Path(), name.Text)

				if _, err := os.Lstat(filename); err == nil {
					dialog.ShowError(
						errors.New("file already exists"),
						ui.MainWindow,
					)
					return
				} else if !errors.Is(err, os.ErrNotExist) {
					dialog.ShowError(err, ui.MainWindow)
					return
				}

				newPassword := widget.NewPasswordEntry()
				newPassword.SetPlaceHolder("Leave empty to preserve")

				passwordDialog := dialog.NewForm(
					"Vault password",
					"Save",
					"Cancel",
					[]*widget.FormItem{
						widget.NewFormItem("Password", newPassword),
					},
					func(ok bool) {
						if !ok {
							newPassword.SetText("")
							return
						}

						secret := []byte(newPassword.Text)
						newPassword.SetText("")

						if len(secret) == 0 {
							secret = append([]byte(nil), ui.vaultKey...)
						}

						serialized, err := ui.vault.Serialize()
						if err != nil {
							clear(secret)
							dialog.ShowError(err, ui.MainWindow)
							return
						}

						progress := widget.NewProgressBarInfinite()
						loading := dialog.NewCustomWithoutButtons(
							"Saving vault",
							container.NewVBox(
								widget.NewLabel("Please wait…"),
								progress,
							),
							ui.MainWindow,
						)
						loading.Show()

						go func() {
							defer clear(serialized)

							err := vault.CreateVault(filename, secret)

							if err == nil {
								err = vault.WriteVault(
									filename,
									secret,
									serialized,
								)

								if err != nil {
									if removeErr := os.Remove(filename); removeErr != nil {
										err = fmt.Errorf(
											"%w; cannot remove incomplete vault: %v",
											err,
											removeErr,
										)
									}
								}
							}

							var loaded *vault.Vault
							if err == nil {
								loaded, err = ui.loadVault(filename, secret)
							}

							fyne.Do(func() {
								progress.Stop()
								loading.Hide()

								if err != nil {
									clear(secret)
									dialog.ShowError(err, ui.MainWindow)
									return
								}

								ui.setVault(filename, secret, loaded)
							})
						}()
					},
					ui.MainWindow,
				)

				passwordDialog.Resize(fyne.NewSize(400, 160))
				passwordDialog.Show()
				ui.MainWindow.Canvas().Focus(newPassword)
			},
			ui.MainWindow,
		)

		nameDialog.Resize(fyne.NewSize(400, 160))
		nameDialog.Show()
		ui.MainWindow.Canvas().Focus(name)
	}, ui.MainWindow)
}

func (ui *UI) onHelp() {
	text := widget.NewMultiLineEntry()
	text.SetText(`Faraday CLI

Every command prompts for the vault password.

Create a vault:
faraday --vault path_to_vault create

Read an entry:
faraday --vault path_to_vault --field fieldNS/fieldName read

Create or update an entry:
faraday --vault path_to_vault --field fieldNS/fieldName --content "some_value" update

Read content from stdin (omit --content):
faraday --vault path_to_vault --field fieldNS/fieldName update < .env

Write an entry to a file:
faraday --vault path_to_vault --field fieldNS/fieldName read > .env

Place all flags before the command.
Entry paths have the format namespace/name.
`)

	// Разрешаем выделение и копирование, но отменяем изменения текста.
	original := text.Text
	text.OnChanged = func(value string) {
		if value != original {
			text.SetText(original)
		}
	}

	d := dialog.NewCustom("Command-line usage", "Close", text, ui.MainWindow)
	d.Resize(fyne.NewSize(800, 500))
	d.Show()
}

func (ui *UI) onEditorChange(text string) {
	if ui.skipChange || ui.editor.Disabled() {
		ui.skipChange = false
		return
	}

	if ui.vault == nil || ui.tree.SelectedNode == "" {
		return
	}

	if err := ui.vault.SetField(ui.tree.SelectedNode, text); err != nil {
		dialog.ShowError(err, ui.MainWindow)
	}
}

func (ui *UI) onNodeSelected(id string) {
	if len(id) > 0 {
		content, err := ui.vault.GetField(id)
		if err != nil {
			ui.editor.Disable()
			ui.editor.SetText("")
			dialog.ShowError(err, ui.MainWindow)
		} else {
			ui.skipChange = true
			ui.editor.Enable()
			ui.editor.SetText(content)
		}
	} else {
		ui.editor.Disable()
		ui.editor.SetText("")
	}
}

func (ui *UI) onCreateNode(prefix string) {
	nodeId := widget.NewEntry()

	d := dialog.NewForm(
		"Create entry",
		"Create",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nodeId),
		},
		func(ok bool) {
			if !ok {
				return
			}

			key := nodeId.Text

			if !vault.ValidKey(key) {
				dialog.ShowError(fmt.Errorf("Node key must be formatted as <namespace>/<name>"), ui.MainWindow)
				return
			} else {
				ui.vault.SetField(key, "")
				ui.tree.Refresh()
			}
		},
		ui.MainWindow,
	)

	d.Resize(fyne.NewSize(400, 160))
	d.Show()
}

func (ui *UI) onDeleteNode(id string) {
	ui.vault.DeleteField(id)
	ui.tree.Refresh()
}

func (ui *UI) loadVault(filename string, secret []byte) (*vault.Vault, error) {
	bytes, err := vault.ReadVault(filename, secret)
	if err != nil {
		return nil, err
	}

	v := &vault.Vault{}
	err = v.Deserialize(bytes)

	if err != nil {
		return nil, err
	} else {
		return v, nil
	}
}

func (ui *UI) setVault(filename string, secret []byte, vault *vault.Vault) {
	clear(ui.vaultKey)

	ui.vault = vault
	ui.vaultPath = filename
	ui.vaultKey = secret

	ui.tree = CreateVaultTree(filepath.Base(filename), ui.vault)
	ui.tree.OnNodeSelected = ui.onNodeSelected
	ui.tree.OnCreateNode = ui.onCreateNode
	ui.tree.OnDeleteNode = ui.onDeleteNode

	ui.editor = widget.NewMultiLineEntry()
	ui.editor.Disable()
	ui.editor.OnChanged = ui.onEditorChange

	ui.content.Leading = ui.tree.Widget
	ui.content.Trailing = ui.editor

	ui.content.Refresh()
}
