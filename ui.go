package main

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/ggwhite/go-masker"
	"github.com/paulosuzart/fgamanager/db"
	"github.com/rivo/tview"
	"log"
	"sync"
	"time"
)

var helpBox *tview.TextView

type count struct {
	totalCount   int
	lock         sync.RWMutex
	newCountChan chan int
}

func (c *count) setTotal(newTotal int) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.totalCount = newTotal
	log.Printf("New count is %v", newTotal)
}

func (c *count) refresh(d time.Duration) {
	for {
		dbCount := db.CountTuples(nil)
		c.setTotal(dbCount)
		c.newCountChan <- dbCount
		time.Sleep(d)
	}
}

type TupleView struct {
	tview.TableContentReadOnly
	// just to avoid going to the database again
	page      *db.LoadResult
	filter    db.Filter
	filterSet bool
}

func newTupleView() *TupleView {
	firstPage := db.Load(0, nil)
	return &TupleView{
		TableContentReadOnly: tview.TableContentReadOnly{},
		page:                 firstPage,
		filter:               db.Filter{},
	}
}

type Action string

const (
	Delete Action = "D"
	Write  Action = "W"
	None   Action = "N"
	Stale  Action = "S"
)

func (a Action) String() string {
	if a == Delete {
		return "D"
	} else if a == Write {
		return "W"
	} else if a == Stale {
		return "S"
	}
	return "N"
}

func (t *TupleView) GetRowCount() int {
	if t.filterSet {
		return db.CountTuples(&t.filter) + 1
	}
	if t.page == nil || t.page.GetTotal() == 0 {
		return 1
	}
	return t.page.GetTotal() + 1
}

func (t *TupleView) GetColumnCount() int {
	// this is fixed
	return 8
}

func (t *TupleView) load(row int) {
	t.filterSet = false
	t.page = db.Load(row, &t.filter)
	log.Printf("Loaded value: %v", t.page)
}

func (t *TupleView) setFilter(filter db.Filter) {
	t.filterSet = true
	t.filter = filter
}

func (t *TupleView) GetCell(row, column int) *tview.TableCell {
	if row == 0 {
		switch column {
		case 0:
			return tview.NewTableCell("USER TYPE             ").SetSelectable(false)
		case 1:
			return tview.NewTableCell("USER ID                                ").SetSelectable(false)
		case 2:
			return tview.NewTableCell("RELATION              ").SetSelectable(false)
		case 3:
			return tview.NewTableCell("OBJECT TYPE              ").SetSelectable(false)
		case 4:
			return tview.NewTableCell("OBJECT ID                                ").SetSelectable(false)
		case 5:
			return tview.NewTableCell("TIMESTAMP \u2191             ").SetSelectable(false)
		case 6:
			return tview.NewTableCell("ACTION  ").SetSelectable(false)
		case 7:
			return tview.NewTableCell("ROW  ").SetSelectable(false)
		default:
			return tview.NewTableCell("Undefined               ").SetSelectable(false)
		}
	}
	if (t.page != nil && (row < t.page.GetLowerBound() || t.page.GetUpperBound() < row)) || t.filterSet {
		t.load(row - 1)
		if t.page == nil || t.page.GetTotal() == 0 || len(t.page.Res) == 0 {
			return nil
		}
		log.Printf("Count: %v. Current bounds: %v-%v. Requested row: %v", len(t.page.Res), t.page.GetLowerBound(), t.page.GetUpperBound(), row)
	}

	index := row - t.page.GetLowerBound()
	tuple := t.page.Res[index].Tuple
	action := t.page.Res[index].Action
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

func createDropdown(label, dropDownType string, getterFunc func() []string) *tview.DropDown {

	dropdown := tview.NewDropDown().
		SetLabel(label)

	dropdown.SetFocusFunc(func() {
		helpBox.SetText("[blue]ENTER:[white] Opens the [orange]" + dropDownType + "[white] dropdown")
	})

	go func() {
		for {
			// we update if nothing is selected and is not open
			if i, _ := dropdown.GetCurrentOption(); i <= 0 && !dropdown.IsOpen() {
				availableTypes := getterFunc()
				options := []string{"Select a " + dropDownType}
				options = append(options, availableTypes...)
				dropdown.SetOptions(options, nil).
					SetCurrentOption(0)

			}
			time.Sleep(5 * time.Second)
		}
	}()

	return dropdown
}

func AddComponents(context context.Context, app *tview.Application) *tview.Grid {
	helpBox = tview.NewTextView()
	helpBox.SetText("Help will appear here").SetTextAlign(tview.AlignCenter).SetDynamicColors(true)

	infoTable := tview.NewTable().SetBorders(false)
	infoTable.SetCell(0, 0, tview.NewTableCell("Watch Active:").
		SetTextColor(tcell.ColorDarkOrange))

	watchView := tview.NewTableCell("??").
		SetTextColor(tcell.ColorLightBlue)

	infoTable.SetCell(1, 0, tview.NewTableCell("Server:").
		SetTextColor(tcell.ColorDarkOrange).SetMaxWidth(60))
	infoTable.SetCell(1, 1, tview.NewTableCell(*apiUrl))

	infoTable.SetCell(1, 2, tview.NewTableCell("StoreId:").
		SetTextColor(tcell.ColorDarkOrange))

	infoTable.SetCell(1, 3, tview.NewTableCell(masker.ID(*storeId)))

	infoTable.SetCell(1, 4, tview.NewTableCell("Continuation Token:").
		SetTextColor(tcell.ColorDarkOrange))
	tokenView := tview.NewTableCell("??").SetMaxWidth(60)

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

	newCount := count{
		newCountChan: make(chan int, 10),
	}
	go newCount.refresh(3 * time.Second)
	tupleView := newTupleView()
	log.Printf("Created table view")

	tupleTable := tview.NewTable().SetContent(tupleView).SetSelectable(true, false).
		SetBorders(false).SetFixed(1, 8)

	tupleTable.SetFocusFunc(func() {
		helpBox.SetText("[green]CTRL-N: [white]Submit new Tuple\n[red]CTRL-D:[white] Mark tuple for [red]deletion[white]\n[blue]Control-Tab:[white] Return to the filter form")
	})
	pages := tview.NewPages()
	pages.SetBorder(true)
	pages.SwitchToPage("help")

	createForm := tview.NewForm().SetHorizontal(true)
	createForm.AddInputField("Tuple", "tuple for creation", 120, nil, nil)
	createForm.AddButton("Create", func() {
		item := createForm.GetFormItem(0).(*tview.InputField)
		log.Printf("Will crate tuple %v", item.GetText())
		go create(context, item.GetText())
		pages.SwitchToPage("help")
		app.SetFocus(tupleTable)
	})

	tupleTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := tupleTable.GetSelection()
		if event.Key() == tcell.KeyCtrlD && row > 0 {
			tuple := tupleView.page.Res[row-tupleView.page.GetLowerBound()].Tuple
			log.Printf("Marking row as deleted %v", tuple.TupleKey)
			db.MarkDeletion(tuple.TupleKey)
			tupleView.load(tupleView.page.GetLowerBound())
		} else if event.Key() == tcell.KeyCtrlN {
			pages.SwitchToPage("create")
			app.SetFocus(createForm)
		}
		return event
	})

	search := tview.NewInputField().
		SetLabel("Filter").
		SetPlaceholder("%budget invoice:%").
		SetFieldWidth(40)

	search.SetFocusFunc(func() {
		helpBox.SetText("[blue]ENTER:[white] triggers the filter with selected options")
	})

	userTypes := createDropdown("User Type", "userType", db.GetUserTypes)
	relations := createDropdown("Relation", "relation", db.GetRelations)
	objectTypes := createDropdown("Object Type", "objectType", db.GetObjectTypes)

	filterForm := tview.NewForm().
		AddFormItem(userTypes).
		AddFormItem(relations).
		AddFormItem(objectTypes).
		AddFormItem(search)
	filterForm.SetBorder(false)
	filterForm.SetHorizontal(true)

	filterForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// if we hit tab at the last item of the form we go to the table
		if i, _ := filterForm.GetFocusedItemIndex(); i == filterForm.GetFormItemCount()-1 {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(tupleTable)
				return event
				// but if we hit enter we just prepare search
			} else if event.Key() == tcell.KeyEnter {
				filter := db.Filter{}
				if searchText := search.GetText(); searchText != "" {
					filter.Search = &searchText
				}
				if i, userType := userTypes.GetCurrentOption(); i > 0 {
					filter.UserType = &userType
				}
				if i, relation := relations.GetCurrentOption(); i > 0 {
					filter.Relation = &relation
				}
				if i, objectType := objectTypes.GetCurrentOption(); i > 0 {
					filter.ObjectType = &objectType
				}
				tupleView.setFilter(filter)
				tupleTable.Select(0, 0)
				app.SetFocus(tupleTable)
				return nil
			}

		}
		return event
	})

	// switch cursor between search and table
	tupleTable.SetDoneFunc(func(key tcell.Key) { app.SetFocus(filterForm) })

	grid := tview.NewGrid().
		SetRows(3, 3, -5, 5).
		SetMinSize(3, 20).
		SetBorders(false).
		AddItem(infoTable, 0, 0, 1, 1, 0, 0, false).
		AddItem(filterForm, 1, 0, 1, 1, 0, 0, true)

	// Layout for screens narrower than 100 cells (menu and side bar are hidden).
	tableFrame := tview.NewFrame(tupleTable)
	tableFrame.SetBorder(true).SetBorderAttributes(tcell.AttrNone)
	grid.AddItem(tableFrame, 2, 0, 1, 1, 0, 0, false)

	pages.AddPage("help", helpBox, true, true).
		AddPage("create", createForm, true, false)

	grid.AddItem(pages, 3, 0, 1, 1, 3, 0, false)

	watchUpdatesChan := make(chan WatchUpdate, 10)
	go func() {
		for {
			select {
			case t := <-watchUpdatesChan:
				app.QueueUpdate(func() {
					if t.Token != nil {
						tokenView.SetText(*t.Token)
					}
					writesView.SetText(fmt.Sprintf("%v", t.Writes))
					deletesView.SetText(fmt.Sprintf("%v", t.Deletes))
					watchView.SetText(fmt.Sprintf("%v", t.WatchEnabled))
					// we decrease one because first line is actually header
				})
			case i := <-newCount.newCountChan:
				log.Printf("New count detected %v", i)
				app.QueueUpdateDraw(func() {
					totalCountView.SetText(fmt.Sprintf("%v", i))
					selectedCountView.SetText(fmt.Sprintf("%v", tupleTable.GetRowCount()-1))
				})

			}
		}
	}()

	go read(context, watchUpdatesChan)
	go deleteMarked(context)

	return grid

}
