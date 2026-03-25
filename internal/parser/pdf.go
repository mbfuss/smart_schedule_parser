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

// PDFParser реализует интерфейс Parser для работы с PDF-файлами.
type PDFParser struct {
	pdfToCsvScriptPath string
}

// NewPDFParser создаёт новый экземпляр PDFParser.
func NewPDFParser(pdfToCsvScriptPath string) *PDFParser {
	return &PDFParser{pdfToCsvScriptPath: pdfToCsvScriptPath}
}

const (
	timeLayout = "15:04"
	MergedCell = "Merged"
)

var weekDayNames = []string{
	Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday,
}

// tableLayout описывает структуру таблицы расписания.
type tableLayout struct {
	headerRow int // строка с именами групп (предшествует первой строке со временем)
	timeCol   int // колонка с временными интервалами
}

// detectLayout сканирует таблицу в поисках первой ячейки с временным
// интервалом и возвращает структуру таблицы. Если ячейка не найдена,
// оба поля равны -1.
func detectLayout(table [][]string) tableLayout {
	for ri, row := range table {
		for ci, cell := range row {
			if _, _, err := parseTimeRow(cell); err == nil {
				return tableLayout{headerRow: ri - 1, timeCol: ci}
			}
		}
	}
	return tableLayout{headerRow: -1, timeCol: -1}
}

// buildGroups создаёт заготовки групп из строки заголовка.
//
// Поддерживает два формата:
//   - одна непустая ячейка → имена групп разделены пробелами внутри неё;
//   - несколько непустых ячеек → каждая ячейка соответствует одной группе.
func buildGroups(headers []string, firstGroupCol int) []resource.Group {
	newGroup := func(name string) resource.Group {
		return resource.Group{Name: name, Schedule: resource.Schedule{Days: []resource.Day{}}}
	}

	nonMergedCount := 0
	for _, cell := range headers[firstGroupCol:] {
		if cell != MergedCell {
			nonMergedCount++
		}
	}

	if nonMergedCount == 1 {
		// Все имена групп упакованы в одну ячейку через пробел («1 бэ», «2 бм оз тд оз»).
		var groups []resource.Group
		for _, name := range strings.Split(headers[firstGroupCol], " ") {
			groups = append(groups, newGroup(name))
		}
		return groups
	}

	var groups []resource.Group
	for _, cell := range headers[firstGroupCol:] {
		if cell != MergedCell {
			groups = append(groups, newGroup(cell))
		}
	}
	return groups
}

// rowTime хранит результат разбора времени из одной строки таблицы.
type rowTime struct {
	From, To         time.Time // основной временной слот
	NextFrom, NextTo time.Time // второй слот (ненулевой, если строка задаёт двойную пару)
	Updated          bool      // false — строка не содержит нового времени
}

// parseRowTime извлекает время из row[timeCol], поддерживая три сценария:
//
//  1. Ячейка содержит один интервал («8.30-10.00»).
//  2. Ячейка содержит два интервала через '\n' («8.30-10.00\n10.10-11.40»).
//  3. Fallback: timeCol определён неверно (например, PDF содержит колонку
//     с номером недели). В этом случае сканируется вся строка.
func parseRowTime(row []string, timeCol int) rowTime {
	if timeCol >= len(row) {
		return rowTime{}
	}
	cell := strings.TrimSpace(row[timeCol])
	if cell == MergedCell || cell == "" {
		return rowTime{} // Merged-ячейка — время для этой строки не меняется
	}

	// Вариант 1: одиночный временной интервал.
	if from, to, err := parseTimeRow(cell); err == nil {
		return rowTime{From: from, To: to, Updated: true}
	}

	// Вариант 2: двойная пара («8.30-10.00\n10.10-11.40»).
	if parts := strings.SplitN(cell, "\n", 2); len(parts) == 2 {
		if from, to, err := parseTimeRow(parts[0]); err == nil {
			if nf, nt, err := parseTimeRow(parts[1]); err == nil {
				return rowTime{From: from, To: to, NextFrom: nf, NextTo: nt, Updated: true}
			}
		}
	}

	// Вариант 3 (fallback): сканируем все ячейки строки.
	for _, c := range row {
		c = strings.TrimSpace(c)
		if c == MergedCell || c == "" {
			continue
		}
		if from, to, err := parseTimeRow(c); err == nil {
			return rowTime{From: from, To: to, Updated: true}
		}
	}

	return rowTime{} // время в строке не найдено
}

// isEmptyRow возвращает true, если все ячейки строки пустые или Merged.
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if s := strings.TrimSpace(cell); s != MergedCell && s != "" {
			return false
		}
	}
	return true
}

// resolveSubject возвращает текст предмета для ячейки группы.
// Если сама ячейка Merged, ищёт ближайшую непустую ячейку левее.
func resolveSubject(row []string, colIdx int) string {
	if colIdx >= len(row) {
		return ""
	}
	if row[colIdx] != MergedCell {
		return strings.ReplaceAll(strings.TrimSpace(row[colIdx]), "\n", " ")
	}
	for offset := 1; colIdx-offset >= 0; offset++ {
		if row[colIdx-offset] != MergedCell {
			return strings.ReplaceAll(strings.TrimSpace(row[colIdx-offset]), "\n", " ")
		}
	}
	return ""
}

// parseTableToGroups превращает двумерную таблицу (из CSV) в расписания групп.
func (p *PDFParser) parseTableToGroups(ctx context.Context, table [][]string) ([]resource.Group, error) {
	if len(table) < 2 {
		return nil, fmt.Errorf("недостаточно строк в таблице")
	}

	layout := detectLayout(table)
	if layout.headerRow < 0 {
		return nil, fmt.Errorf("не найдена колонка с временем занятий")
	}

	headers := table[layout.headerRow]
	if len(headers) < 3 {
		return nil, fmt.Errorf("ожидалось минимум 3 колонки (День, Время, Группа…)")
	}

	groupColOffset := layout.timeCol + 1
	groups := buildGroups(headers, groupColOffset)

	// ── состояние обхода ──────────────────────────────────────────────────────
	dayIndex := -1               // текущий индекс в weekDayNames; -1 = день ещё не создан
	var curFrom, curTo time.Time // текущий временной слот
	var prevFrom time.Time       // предыдущий слот — для обнаружения смены дня

	dataStart := layout.headerRow + 1
	for i, row := range table[dataStart:] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		rowIdx := dataStart + i

		// nextRow нужен parseWeekType для определения числитель/знаменатель.
		// Если текущая строка пустая и следующая — последняя, это хвостовой мусор.
		var nextRow []string
		if rowIdx+1 < len(table) {
			if rowIdx+1 == len(table)-1 && isEmptyRow(row) {
				break
			}
			nextRow = table[rowIdx+1]
		}

		// ── обновление времени ────────────────────────────────────────────────
		rt := parseRowTime(row, layout.timeCol)
		if rt.Updated {
			prevFrom = curFrom
			curFrom = rt.From
			curTo = rt.To
		}

		// ── обнаружение смены дня ─────────────────────────────────────────────
		// Проверяем ТОЛЬКО при реальном обновлении времени: Merged-строки
		// (знаменатель той же пары) не должны создавать ложные дни.
		if rt.Updated && curFrom.Before(prevFrom) {
			dayIndex++
			if dayIndex >= len(weekDayNames) {
				return nil, fmt.Errorf("количество дней превысило %d", len(weekDayNames))
			}
			for gi := range groups {
				groups[gi].Schedule.Days = append(groups[gi].Schedule.Days, resource.Day{
					Name: weekDayNames[dayIndex],
				})
			}
		}

		// Строка с двойной парой: следующие строки относятся ко второму слоту.
		if !rt.NextFrom.IsZero() {
			curFrom = rt.NextFrom
			curTo = rt.NextTo
		}

		// ── запись занятий для каждой группы ─────────────────────────────────
		for gi := range groups {
			colIdx := groupColOffset + gi
			if colIdx >= len(row) || dayIndex < 0 {
				continue
			}

			weekType := parseWeekType(row, nextRow, layout.timeCol, colIdx)

			// Пропускаем пустые ячейки, кроме случая когда это числитель для
			// группы gi>0 (занятие для gi=0 уже добавлено, остальные — Merged-копии).
			isSharedLecture := weekType == resource.WeekNumerator && gi > 0
			if !isSharedLecture && (row[colIdx] == MergedCell || row[colIdx] == "") {
				continue
			}

			subject := resolveSubject(row, colIdx)
			if subject == "" {
				continue
			}

			lesson := resource.Lesson{
				TimeFrom: curFrom.Format(timeLayout),
				TimeTo:   curTo.Format(timeLayout),
				Subject:  subject,
			}
			if weekType != resource.WeekNone {
				lesson.Week = &weekType
			}
			groups[gi].Schedule.Days[dayIndex].Lessons = append(
				groups[gi].Schedule.Days[dayIndex].Lessons, lesson,
			)
		}
	}

	deleteEmptyDays(groups)
	return groups, nil
}

// deleteEmptyDays удаляет дни без занятий из расписания каждой группы.
func deleteEmptyDays(groups []resource.Group) {
	for gi := range groups {
		days := groups[gi].Schedule.Days
		for di := len(days) - 1; di >= 0; di-- {
			if days[di].Lessons == nil {
				days = append(days[:di], days[di+1:]...)
			}
		}
		groups[gi].Schedule.Days = days
	}
}

// parseWeekType определяет тип недели (числитель/знаменатель/нет) для ячейки группы.
//
//   - Знаменатель: колонка времени в текущей строке — Merged (вторая строка пары).
//   - Числитель: следующая строка является знаменателем для той же группы.
//   - Нет: занятие относится к обеим неделям.
func parseWeekType(row, nextRow []string, timeCol, groupCol int) resource.WeekType {
	if timeCol >= len(row) || groupCol >= len(row) {
		return resource.WeekNone
	}
	if row[timeCol] == MergedCell {
		return resource.WeekDenominator
	}
	if nextRow != nil &&
		timeCol < len(nextRow) &&
		groupCol < len(nextRow) &&
		nextRow[timeCol] == MergedCell &&
		nextRow[groupCol] != MergedCell {
		return resource.WeekNumerator
	}
	return resource.WeekNone
}

// parseTimeRow разбирает строку с временным интервалом («8.30-10.00», «8:30-10:00»)
// и возвращает время начала и окончания. Разделители между часами и минутами,
// а также между двумя временами могут быть любыми не-цифровыми символами.
func parseTimeRow(timeCell string) (time.Time, time.Time, error) {
	timeCell = strings.ReplaceAll(timeCell, "\n", " ")
	re := regexp.MustCompile(`(\d{1,2})\D(\d{2})\D(\d{1,2})\D(\d{2})`)
	matches := re.FindAllStringSubmatch(timeCell, -1)
	if len(matches) != 1 || len(matches[0]) != 5 {
		return time.Time{}, time.Time{}, fmt.Errorf("не корректное значение ячейки времени: %q", timeCell)
	}
	m := matches[0] // m[1]=ч_нач, m[2]=мин_нач, m[3]=ч_кон, m[4]=мин_кон

	parseHHMM := func(hh, mm string) (time.Time, error) {
		hour, err := strconv.Atoi(hh)
		if err != nil {
			return time.Time{}, fmt.Errorf("неверный час %q: %w", hh, err)
		}
		min0, err := strconv.Atoi(mm)
		if err != nil {
			return time.Time{}, fmt.Errorf("неверные минуты %q: %w", mm, err)
		}
		return time.Date(0, 1, 1, hour, min0, 0, 0, time.UTC), nil
	}

	start, err := parseHHMM(m[1], m[2])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("время начала: %w", err)
	}
	end, err := parseHHMM(m[3], m[4])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("время окончания: %w", err)
	}
	return start, end, nil
}

// ParsePDF парсит PDF-файл и возвращает список групп с расписаниями.
// Паники внутри парсера перехватываются и возвращаются как ошибки.
func (p *PDFParser) ParsePDF(ctx context.Context, filePath string) (groups []resource.Group, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("паника [filePath=%q]: %v: %w", filePath, r, err)
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

// pdfToTable запускает Python-скрипт, который конвертирует PDF в CSV,
// и возвращает содержимое таблицы в виде двумерного среза строк.
func (p *PDFParser) pdfToTable(ctx context.Context, pdfPath string) ([][]string, error) {
	cmd := exec.CommandContext(ctx, "python3", p.pdfToCsvScriptPath, pdfPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("выполнение python скрипта конвертации pdf в csv отменено: %w", ctx.Err())
		}
		return nil, fmt.Errorf("выполнение python скрипта конвертации pdf в csv [stdout=%s, stderr=%s]: %w",
			stdout.String(), stderr.String(), err)
	}

	records, err := csv.NewReader(&stdout).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга csv: %w", err)
	}

	return records, nil
}
