package main

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"log"
	"manager/db"
)

type TupleView struct {
	tview.TableContentReadOnly
	// just to avoid going to the database again
	query *string
	page  *db.LoadResult
}

type Action string

const (
	Delete Action = "D"
	Write  Action = "W"
	None   Action = "N"
)

func (a Action) String() string {
	if a == Delete {
		return "D"
	} else if a == Write {
		return "W"
	}
	return "N"
}

func (t *TupleView) GetRowCount() int {
	return db.CountTuples(t.query) + 1
}

func (t *TupleView) GetColumnCount() int {
	// this is fixed
	return 8
}

func (t *TupleView) load(row int) {
	t.page = db.Load(row, t.query)
}

func (t *TupleView) GetCell(row, column int) *tview.TableCell {
	if row == 0 {
		switch column {
		case 0:
			return tview.NewTableCell("USER TYPE             ")
		case 1:
			return tview.NewTableCell("USER ID                                ")
		case 2:
			return tview.NewTableCell("RELATION              ")
		case 3:
			return tview.NewTableCell("OBJECT TYPE              ")
		case 4:
			return tview.NewTableCell("OBJECT ID                                ")
		case 5:
			return tview.NewTableCell("TIMESTAMP \u2191             ")
		case 6:
			return tview.NewTableCell("ACTION  ")
		case 7:
			return tview.NewTableCell("ROW  ")
		default:
			return tview.NewTableCell("Undefined               ")
		}
	}

	if t.page == nil || t.page.LowerBound > row || t.page.UpperBound < row || (t.query != nil && t.page.Query != *t.query) {
		t.load(row - 1)
		log.Printf("Current bounds: %v-%v. Requested row: %v", t.page.LowerBound, t.page.UpperBound, row)
	}

	if len(t.page.Res) == 0 {
		return nil
	}

	tuple := t.page.Res[row-t.page.LowerBound].Tuple
	action := t.page.Res[row-t.page.LowerBound].Action
	switch column {
	case 0:
		return tview.NewTableCell(tuple.UserType).SetTextColor(tcell.ColorLightCyan)
	case 1:
		return tview.NewTableCell(tuple.UserId).SetTextColor(tcell.ColorLightCyan)
	case 2:
		return tview.NewTableCell(tuple.Relation).SetTextColor(tcell.ColorLightCyan)
	case 3:
		return tview.NewTableCell(tuple.ObjectType).SetTextColor(tcell.ColorLightCyan)
	case 4:
		return tview.NewTableCell(tuple.ObjectId).SetTextColor(tcell.ColorLightCyan)
	case 5:
		return tview.NewTableCell(tuple.Timestamp.String()).SetTextColor(tcell.ColorLightCyan)
	case 6:
		cell := tview.NewTableCell(action)
		if action == Delete.String() {
			cell.SetTextColor(tcell.ColorLightCoral)
		} else if action == Write.String() {
			cell.SetTextColor(tcell.ColorLightGreen)
		}
		return cell
	case 7:
		return tview.NewTableCell(fmt.Sprintf("%v", tuple.Row)).SetTextColor(tcell.ColorLightCyan)
	default:
		return tview.NewTableCell("Undefined").SetTextColor(tcell.ColorRed)
	}
}

func userTypesDropdown() *tview.DropDown {
	availableTypes := db.GetUserTypes()
	options := []string{"Select a User Type"}
	options = append(options, availableTypes...)
	dropdown := tview.NewDropDown().
		SetLabel("User Types").
		SetOptions(options, nil).
		SetCurrentOption(0)

	return dropdown
}
func AddComponents(context context.Context, app *tview.Application) *tview.Grid {
	infoTable := tview.NewTable().SetBorders(false)
	infoTable.SetCell(0, 0, tview.NewTableCell("Watch Active:").
		SetTextColor(tcell.ColorDarkOrange))

	watchView := tview.NewTableCell("??").
		SetTextColor(tcell.ColorLightBlue)

	infoTable.SetCell(1, 0, tview.NewTableCell("Server:").
		SetTextColor(tcell.ColorDarkOrange))
	infoTable.SetCell(1, 1, tview.NewTableCell(apiUrl))

	infoTable.SetCell(1, 2, tview.NewTableCell("StoreId:").
		SetTextColor(tcell.ColorDarkOrange))
	infoTable.SetCell(1, 3, tview.NewTableCell(storeId))

	infoTable.SetCell(1, 4, tview.NewTableCell("Continuation Token:").
		SetTextColor(tcell.ColorDarkOrange))
	tokenView := tview.NewTableCell("??")

	infoTable.SetCell(2, 0, tview.NewTableCell("W:").
		SetTextColor(tcell.ColorLightGreen))
	writesView := tview.NewTableCell("??")

	infoTable.SetCell(2, 2, tview.NewTableCell("D:").
		SetTextColor(tcell.ColorRed))
	deletesView := tview.NewTableCell("??")

	infoTable.SetCell(2, 4, tview.NewTableCell("Total count:").
		SetTextColor(tcell.ColorDarkOrange))
	totalCountView := tview.NewTableCell("??")

	infoTable.SetCell(2, 6, tview.NewTableCell("Filtered count:").
		SetTextColor(tcell.ColorDarkOrange))
	selectedCountView := tview.NewTableCell("??")

	infoTable.SetCell(0, 1, watchView)
	infoTable.SetCell(1, 5, tokenView)
	infoTable.SetCell(2, 1, writesView)
	infoTable.SetCell(2, 3, deletesView)
	infoTable.SetCell(2, 5, totalCountView)
	infoTable.SetCell(2, 7, selectedCountView)

	tuples := &TupleView{}

	tupleTable := tview.NewTable().SetContent(tuples).SetSelectable(true, false).
		SetBorders(false).SetFixed(1, 8)
	tupleTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := tupleTable.GetSelection()
		if event.Key() == tcell.KeyCtrlD && row > 1 {
			tuple := tuples.page.Res[row-tuples.page.LowerBound].Tuple
			log.Printf("Marking row as deleted %v", tuple.TupleKey)
			db.MarkDeletion(tuple.TupleKey)
			tuples.load(tuples.page.LowerBound)
		}
		return event
	})

	search := tview.NewInputField().
		SetLabel("Filter: ").
		SetPlaceholder("%budget invoice:%").
		SetFieldWidth(40)

	userTypes := userTypesDropdown()

	filterForm := tview.NewForm().
		AddFormItem(userTypes).
		AddFormItem(search)

	filterForm.SetHorizontal(true)

	filterForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// if we hit tab at the last item of the form we go to the table
		if i, _ := filterForm.GetFocusedItemIndex(); i == filterForm.GetFormItemCount()-1 && event.Key() == tcell.KeyTab {
			app.SetFocus(tupleTable)
		}
		return event
	})

	// switch cursor between search and table
	tupleTable.SetDoneFunc(func(key tcell.Key) { app.SetFocus(filterForm) })

	// on enter we set search filter
	search.SetDoneFunc(func(key tcell.Key) {
		log.Printf("Done search %v", key)
		if key == tcell.KeyEnter {
			txt := search.GetText()
			tuples.query = &txt
			log.Printf("Adding search value %v", *tuples.query)
		} //else if key == tcell.KeyTab || key == tcell.KeyBacktab {
		//	app.SetFocus(tupleTable)
		//}
	})

	grid := tview.NewGrid().
		SetRows(3, -1).
		SetMinSize(3, 20).
		SetBorders(true).
		AddItem(infoTable, 0, 0, 1, 1, 0, 0, false).
		AddItem(filterForm, 1, 0, 2, 1, 0, 0, true)

	// Layout for screens narrower than 100 cells (menu and side bar are hidden).
	grid.AddItem(tupleTable, 3, 0, 10, 1, 0, 0, false)
	watchUpdatesChan := make(chan WatchUpdate)
	go func() {
		for {
			t := <-watchUpdatesChan
			app.QueueUpdateDraw(func() {
				if t.Token != nil {
					tokenView.SetText(*t.Token)
				}
				writesView.SetText(fmt.Sprintf("%v", t.Writes))
				deletesView.SetText(fmt.Sprintf("%v", t.Deletes))
				watchView.SetText(fmt.Sprintf("%v", t.WatchEnabled))
				totalCountView.SetText(fmt.Sprintf("%v", db.CountTuples(nil)))
				// we decrease one because first line is actually header
				selectedCountView.SetText(fmt.Sprintf("%v", tupleTable.GetRowCount()-1))
			})
		}
	}()

	go read(context, watchUpdatesChan)

	return grid

}
