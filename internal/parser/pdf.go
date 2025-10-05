package parser

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"smart_schedule_parser/internal/resource"
)

// PDFParser реализует интерфейс Parser для работы с PDF-файлами
type PDFParser struct {
	pdfToCsvScriptPath string
}

// TableCell — ячейка таблицы
type TableCell struct {
	X float64
	Y float64
	T string
}

// NewPDFParser создает новый экземпляр PDFParser
func NewPDFParser(pdfToCsvScriptPath string) *PDFParser {
	return &PDFParser{
		pdfToCsvScriptPath: pdfToCsvScriptPath,
	}
}

const (
	timeLayout = "15:04"
	MergedCell = "Merged"
)

var (
	weekDayNames = []string{
		Monday,
		Tuesday,
		Wednesday,
		Thursday,
		Friday,
		Saturday,
		Sunday,
	}
)

// parseTableToGroups превращает таблицу в расписания групп
func (p *PDFParser) parseTableToGroups(ctx context.Context, table [][]string) ([]resource.Group, error) {
	if len(table) < 2 {
		return nil, fmt.Errorf("недостаточно строк в таблице")
	}

	tableFirstRow := -1
	timeColumn := -1
	// Ищем первую строку и колонку со временем
	for rowIndex, row := range table {
		for cellIndex, cell := range row {
			_, _, err := parseTimeRow(cell)
			if err == nil {
				tableFirstRow = rowIndex - 1
				timeColumn = cellIndex
				break
			}
		}
		if tableFirstRow != -1 {
			break
		}
	}
	firstRowGroupColumn := timeColumn + 1
	lessonColumn := timeColumn + 1

	// Названия групп
	headers := table[tableFirstRow]
	if len(headers) < 3 {
		return nil, fmt.Errorf("ожидалось минимум 3 колонки (Дни, Часы, Группа1 Группа2...)")
	}

	// создаём группы
	var groups []resource.Group
	for i := 0; i < len(headers)-firstRowGroupColumn; i++ { // начиная с 3-й колонки
		groupsCell := headers[firstRowGroupColumn+i]
		if groupsCell == MergedCell {
			continue
		}
		for _, gName := range strings.Split(groupsCell, " ") {
			groups = append(groups, resource.Group{
				Name: gName,
				Schedule: resource.Schedule{
					Days: []resource.Day{},
				},
			})
		}
	}

	dayIndex := -1
	lessonIndex := -1

	previousDayIndex := -1
	var previousTimeFrom time.Time
	var timeFrom, timeTo time.Time
	weekDayStartRowIndex := tableFirstRow + 1
	for i, row := range table[weekDayStartRowIndex:] {
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // прерываем если контекст отменён
		default:
		}

		rowIndex := weekDayStartRowIndex + i
		var nextRow []string
		if rowIndex+1 <= len(table)-1 {
			if rowIndex+1 == len(table)-1 {
				isEmpty := true
				for _, cell := range row {
					cell = strings.TrimSpace(cell)
					if cell != MergedCell && cell != "" {
						isEmpty = false
						break
					}
				}
				if isEmpty {
					break
				}
			}
			nextRow = table[rowIndex+1]
		} else {
			nextRow = nil
		}

		rowTimesCell := strings.TrimSpace(row[timeColumn])
		if rowTimesCell != MergedCell && rowTimesCell != "" {
			var err error
			previousTimeFrom = timeFrom
			timeFrom, timeTo, err = parseTimeRow(rowTimesCell)
			if err != nil {
				return nil, fmt.Errorf("парсинг времени [rows[%d]=%v]: %s", rowIndex, row, rowTimesCell)
			}
			// Если время меньше относительно предыдущего - новый день
			if timeFrom.Before(previousTimeFrom) {
				previousDayIndex = dayIndex
				dayIndex++
				for groupIndex := range groups {
					groups[groupIndex].Schedule.Days = append(groups[groupIndex].Schedule.Days, resource.Day{
						Name: weekDayNames[dayIndex],
					})
				}
			}
		}

		isNewDay := previousDayIndex != dayIndex
		lessonIndex++
		if isNewDay {
			lessonIndex = 0
		}
		for groupIndex := range groups {
			groupLessonColumn := lessonColumn + groupIndex
			weekType := parseWeekType(row, nextRow, timeColumn, groupLessonColumn)

			if (weekType == resource.WeekNumerator && groupIndex > 0) || (row[groupLessonColumn] != MergedCell && row[groupLessonColumn] != "") {
				var subjectCellValue string
				if row[groupLessonColumn] != MergedCell {
					subjectCellValue = row[groupLessonColumn]
				} else {
					// Если ячейка предмета группы объединена, то берём из первой не объединённой левой ячейки
					offset := 1
					for row[groupLessonColumn-offset] == MergedCell {
						offset++
					}
					subjectCellValue = row[groupLessonColumn-offset]
				}
				subject := strings.ReplaceAll(strings.TrimSpace(subjectCellValue), "\n", " ")
				if subject == "" {
					continue
				}
				lesson := resource.Lesson{
					TimeFrom: timeFrom.Format(timeLayout),
					TimeTo:   timeTo.Format(timeLayout),
					Subject:  subject,
				}
				if weekType != resource.WeekNone {
					lesson.Week = &weekType
				}
				groups[groupIndex].Schedule.Days[dayIndex].Lessons = append(groups[groupIndex].Schedule.Days[dayIndex].Lessons, lesson)
			}
		}
	}

	deleteEmptyDays(groups)
	return groups, nil
}

// deleteEmptyDays удаляет дни без занятий
func deleteEmptyDays(groups []resource.Group) {
	for groupIndex := range groups {
		days := groups[groupIndex].Schedule.Days
		for dayIndex := len(days) - 1; dayIndex >= 0; dayIndex-- {
			if days[dayIndex].Lessons == nil {
				days = append(days[:dayIndex], days[dayIndex+1:]...)
			}
		}
		groups[groupIndex].Schedule.Days = days
	}
}

// parseWeekType возвращает тип недели для текущего занятия группы
func parseWeekType(row, nextRow []string, timeColumn, groupLessonColumn int) resource.WeekType {
	if row[timeColumn] == MergedCell {
		return resource.WeekDenominator
	} else if nextRow != nil &&
		nextRow[timeColumn] == MergedCell &&
		nextRow[groupLessonColumn] != MergedCell {
		return resource.WeekNumerator
	}
	return resource.WeekNone
}

// parseTimeRow парсит строку с временем
// и возвращает начало и конец как time.Time с произвольной датой.
func parseTimeRow(timeCell string) (time.Time, time.Time, error) {
	timeCell = strings.ReplaceAll(timeCell, "\n", " ")
	re := regexp.MustCompile(`(\d{1,2})\D(\d{2})\D(\d{1,2})\D(\d{2})`)
	parts := re.FindAllStringSubmatch(timeCell, -1)
	if len(parts) != 1 || len(parts[0]) != 5 {
		return time.Time{}, time.Time{}, fmt.Errorf("не корректное значение ячейки времени: %q", timeCell)
	}
	matches := parts[0]

	// matches[1]=часы начала, [2]=минуты начала, [3]=часы конца, [4]=минуты конца
	parse := func(hh, mm string) (time.Time, error) {
		hour, err := strconv.Atoi(hh)
		if err != nil {
			return time.Time{}, fmt.Errorf("ошибка парсинга часа %q: %w", hh, err)
		}
		min, err := strconv.Atoi(mm)
		if err != nil {
			return time.Time{}, fmt.Errorf("ошибка парсинга минут %q: %w", mm, err)
		}
		return time.Date(0, 1, 1, hour, min, 0, 0, time.UTC), nil
	}

	start, err := parse(matches[1], matches[2])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("не удалось распарсить время начала занятия [parts[0]=%q]: %w", parts[0], err)
	}

	end, err := parse(matches[3], matches[4])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("не удалось распарсить время начала занятия [parts[1]=%q]: %w", parts[1], err)
	}

	return start, end, nil
}

// ParsePDF — теперь полностью рабочий метод
func (p *PDFParser) ParsePDF(ctx context.Context, filePath string) (groups []resource.Group, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = fmt.Errorf("паника [filePath=%q]: %v: %w", filePath, e, err)
		}
	}()
	table, err := p.pdfToTable(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("парсинг pdf в таблицу [filePath=%q]: %w", filePath, err)
	}

	groups, err = p.parseTableToGroups(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("парсинг таблицы в расписание групп [filePath=%q]: %w", filePath, err)
	}

	return groups, nil
}

// pdfToTable конвертирует таблицы из PDF в двумерный массив строк
func (p *PDFParser) pdfToTable(ctx context.Context, pdfPath string) ([][]string, error) {
	cmd := exec.CommandContext(ctx, "python3", p.pdfToCsvScriptPath, pdfPath)

	// захватываем stdout и stderr Python-скрипта
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("выполнение python скрипта конвертации pdf в csv отменено: %w", ctx.Err())
		}
		return nil, fmt.Errorf("выполнение python скрипта конвертации pdf в csv [stdout=%s, stderr=%s]: %w", stdout.String(), stderr.String(), err)
	}

	// читаем CSV прямо из stdout
	reader := csv.NewReader(&stdout)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга csv: %w", err)
	}

	return records, nil
}
