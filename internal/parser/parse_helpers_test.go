package parser

import (
	"context"
	"testing"

	"smart_schedule_parser/internal/resource"
)

// ---------------------------------------------------------------------------
// parseTimeRow
// ---------------------------------------------------------------------------

func TestParseTimeRow_ValidFormats(t *testing.T) {
	cases := []struct {
		input    string
		wantFrom string
		wantTo   string
	}{
		{"8.30-10.00", "08:30", "10:00"},
		{"10.10-11.40", "10:10", "11:40"},
		{"13.40-15.10", "13:40", "15:10"},
		{"8:30-10:00", "08:30", "10:00"}, // двоеточие как разделитель
	}
	for _, tc := range cases {
		from, to, err := parseTimeRow(tc.input)
		if err != nil {
			t.Errorf("parseTimeRow(%q) неожиданная ошибка: %v", tc.input, err)
			continue
		}
		gotFrom := from.Format("15:04")
		gotTo := to.Format("15:04")
		if gotFrom != tc.wantFrom || gotTo != tc.wantTo {
			t.Errorf("parseTimeRow(%q): хотели %s-%s, получили %s-%s",
				tc.input, tc.wantFrom, tc.wantTo, gotFrom, gotTo)
		}
	}
}

func TestParseTimeRow_Invalid(t *testing.T) {
	cases := []string{
		"",
		"Merged",
		"лек. Математика Иванов 123",
		"пр.з. Иностранный язык\nЛарцева К.А. 450",
	}
	for _, s := range cases {
		_, _, err := parseTimeRow(s)
		if err == nil {
			t.Errorf("parseTimeRow(%q): ожидалась ошибка, но её не было", s)
		}
	}
}

// TestParseTimeRow_CyrillicTypo проверяет, что опечатка «л» вместо «1»
// (реальный кейс из логов: «10.10-11.4л0») корректно возвращает ошибку.
func TestParseTimeRow_CyrillicTypo(t *testing.T) {
	_, _, err := parseTimeRow("10.10-11.4л0")
	if err == nil {
		t.Error("parseTimeRow(\"10.10-11.4л0\"): ожидалась ошибка на кириллической опечатке")
	}
}

// ---------------------------------------------------------------------------
// parseTableToGroups — Short rows (Bug 1)
// ---------------------------------------------------------------------------

// TestParseTableToGroups_ShortRowNoPanic воспроизводит «panic: index out of range [3] with length 3»:
// заголовок обещает 2 группы, но некоторые строки данных содержат только 3 колонки.
func TestParseTableToGroups_ShortRowNoPanic(t *testing.T) {
	p := &PDFParser{}

	table := [][]string{
		// строка-заголовок (tableFirstRow): day | time | Группа1 | Группа2
		{"День", "Время", "Б-Э-101", "Б-БИ-101"},
		// первая строка данных: полная
		{"Понедельник", "8.30-10.00", "лек. Математика Иванов 123", "лек. Физика Петров 456"},
		// короткая строка — только 3 колонки (воспроизводит баг)
		{"Merged", "Merged", "лек. Химия Сидоров 789"},
		// ещё одна полная строка
		{"Merged", "10.10-11.40", "пр.з. Математика Иванов 123", "пр.з. Физика Петров 456"},
	}

	// Функция не должна паниковать и не должна возвращать ошибку.
	groups, err := p.parseTableToGroups(context.Background(), table)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("ожидались группы, получено 0")
	}
}

// ---------------------------------------------------------------------------
// parseTableToGroups — Wrong time column fallback (Bug 2)
// ---------------------------------------------------------------------------

// TestParseTableToGroups_TimeColumnFallback воспроизводит ситуацию, когда timeColumn
// определяется как 2 (из-за наличия колонки «номер недели» в первой строке данных),
// а последующие строки имеют время в колонке 1.
// Парсер должен найти время через fallback-сканирование строки.
func TestParseTableToGroups_TimeColumnFallback(t *testing.T) {
	p := &PDFParser{}

	// Строка 0: формальный заголовок (tableFirstRow будет = 0)
	// Строка 1: первая строка с временем — но оно в колонке 2 (week indicator в col 1)
	//            → timeColumn детектируется как 2
	// Строка 2: строка без week-indicator — время в колонке 1, col 2 — занятие
	//            → row[timeColumn=2] = занятие → должен сработать fallback
	table := [][]string{
		{"", "", "", "Б-Э-101"}, // header (tableFirstRow=0)
		{"Понедельник", "1 нед.", "8.30-10.00", "лек. Математика Иванов 123"}, // time at col 2
		{"Merged", "8.30-10.00", "пр.з. Математика Иванов 123"},               // time at col 1 (short row)
	}

	groups, err := p.parseTableToGroups(context.Background(), table)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("ожидались группы, получено 0")
	}
}

// ---------------------------------------------------------------------------
// parseWeekType — Bounds checks (Bug 1)
// ---------------------------------------------------------------------------

func TestParseWeekType_ShortRowNoPanic(t *testing.T) {
	row := []string{"Merged", "Merged"} // только 2 колонки
	nextRow := []string{"Merged", "8.30-10.00"}

	// timeColumn=1, groupLessonColumn=3 — явно за пределами row
	result := parseWeekType(row, nextRow, 1, 3)
	if result != resource.WeekNone {
		t.Errorf("ожидался WeekNone для короткой строки, получено %v", result)
	}
}

// ---------------------------------------------------------------------------
// parseTableToGroups — Merged time rows must NOT create extra days (Bug: weekDayNames[7])
// ---------------------------------------------------------------------------

// TestParseTableToGroups_MergedTimeCellNoDuplicateDay проверяет, что Merged-ячейки
// в колонке времени не создают ложные дни. Это воспроизводит баг
// «panic: index out of range [7] with length 7» (weekDayNames переполнялся).
func TestParseTableToGroups_MergedTimeCellNoDuplicateDay(t *testing.T) {
	p := &PDFParser{}

	// Расписание на 3 дня. Каждая пара имеет числитель и знаменатель (Merged в колонке времени).
	// Без фикса каждый Merged создаёт ложный день и dayIndex выходит за bounds weekDayNames.
	table := [][]string{
		{"", "Время", "Б-Э-101"}, // заголовок (tableFirstRow=0)
		// Понедельник
		{"Понедельник", "8.30-10.00", "лек. A"},
		{"Merged", "Merged", "пр. A den"}, // знаменатель — ложный день без фикса
		{"Merged", "10.10-11.40", "лек. B"},
		{"Merged", "Merged", "пр. B den"},
		// Вторник
		{"Вторник", "8.30-10.00", "лек. C"},
		{"Merged", "Merged", "пр. C den"},
		{"Merged", "10.10-11.40", "лек. D"},
		{"Merged", "Merged", "пр. D den"},
		// Среда
		{"Среда", "8.30-10.00", "лек. E"},
		{"Merged", "Merged", "пр. E den"},
		{"Merged", "10.10-11.40", "лек. F"},
		{"Merged", "Merged", "пр. F den"},
	}

	groups, err := p.parseTableToGroups(context.Background(), table)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("ожидались группы, получено 0")
	}
	days := groups[0].Schedule.Days
	// Должно быть ровно 3 дня, а не 3 + ложные
	if len(days) != 3 {
		t.Errorf("ожидалось 3 дня, получено %d: %v", len(days), func() []string {
			var names []string
			for _, d := range days {
				names = append(names, d.Name)
			}
			return names
		}())
	}
}

// TestParseTableToGroups_NoPanicAtWeekDayBounds проверяет, что парсер не паникует
// при расписании на все 7 дней с Merged-строками (знаменатель) в каждой паре.
func TestParseTableToGroups_NoPanicAtWeekDayBounds(t *testing.T) {
	p := &PDFParser{}

	table := [][]string{
		{"", "Время", "Г-1"}, // заголовок
		{"Пн", "8.30-10.00", "A"}, {"Merged", "Merged", "A den"},
		{"Вт", "8.30-10.00", "B"}, {"Merged", "Merged", "B den"},
		{"Ср", "8.30-10.00", "C"}, {"Merged", "Merged", "C den"},
		{"Чт", "8.30-10.00", "D"}, {"Merged", "Merged", "D den"},
		{"Пт", "8.30-10.00", "E"}, {"Merged", "Merged", "E den"},
		{"Сб", "8.30-10.00", "F"}, {"Merged", "Merged", "F den"},
		{"Вс", "8.30-10.00", "G"}, {"Merged", "Merged", "G den"},
	}

	// Не должно паниковать
	_, err := p.parseTableToGroups(context.Background(), table)
	if err != nil {
		// Ошибка допустима (например, "количество дней превысило 7"),
		// но не паника.
		t.Logf("parseTableToGroups вернул ошибку (ожидаемо при 7 реальных днях): %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseTableToGroups — Context cancellation
// ---------------------------------------------------------------------------

func TestParseTableToGroups_CanceledContext(t *testing.T) {
	p := &PDFParser{}

	table := [][]string{
		{"День", "Время", "Б-Э-101"},
		{"Понедельник", "8.30-10.00", "лек. Математика Иванов 123"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.parseTableToGroups(ctx, table)
	if err == nil {
		t.Fatal("ожидалась ошибка при отменённом контексте")
	}
}
