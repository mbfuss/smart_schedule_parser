package parser_test

import (
	"context"
	"errors"
	"testing"

	"smart_schedule_parser/internal/parser"
	"smart_schedule_parser/internal/resource"
)

const pdfToCsvScriptPath = "../../scripts/pdf2csv.py"
const validTestFilePdf = "../../test/pdf/1758281204_1 э.pdf"

var (
	numerator   = resource.WeekNumerator
	denominator = resource.WeekDenominator
)

func TestPDFParser_ParsePDF_Success(t *testing.T) {
	const (
		firstFrom  = "08.30"
		firstTo    = "10.00"
		secondFrom = "10.10"
		secondTo   = "11.40"
		thirdFrom  = "12.00"
		thirdTo    = "13.30"
		fourFrom   = "13.40"
		fourTo     = "15.10"
		fiveFrom   = "15.20"
		fiveTo     = "16.50"
	)
	expectedGroups := []resource.Group{
		{
			Name: "Б-Э-101",
			Schedule: resource.Schedule{
				Days: []resource.Day{
					{
						Name: parser.Monday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "лек. ФИЗИЧЕСКАЯ КУЛЬТУРА И СПОРТ ФИЗИЧЕСКАЯ КУЛЬТУРА И СПОРТ доц. Кузьмин А.М. 422",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &denominator,
								Subject:  "пр.з. Физическая культура и сорт Кузьмин 422",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "лек. ИСТОРИЯ РОССИИ ИСТОРИЯ РОССИИ проф Шалаева Н.В. 422",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "пр. з. История России Шалаева 422",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     &numerator,
								Subject:  "пр. з. История России Шалаева 422",
							},
						},
					},
					{
						Name: parser.Tuesday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &numerator,
								Subject:  "лек. ИСТОРИЯ МИРОВОЙ КУЛЬТУРЫ ст.пр. Аленичева Н.В. 314",
							},
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &denominator,
								Subject:  "пр.з. Русский язык и культура речи Веселкова 452",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "пр.з. История мировой культуры Аленичева 314",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &denominator,
								Subject:  "пр.з. Русский язык и культура речи Веселкова 452",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "Экономическая культура доц. Васильева Е.А. (5 лек.), Ерзова (5 пр.з.) 232",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     &denominator,
								Subject:  "пр.з .Иностанный язык Раздобарова, Бормосова 450, 528",
							},
						},
					},
					{
						Name: parser.Wednesday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &numerator,
								Subject:  "лек. ОСНОВЫ РОССИЙСКОЙ ГОСУДАРСТВЕННОСТИ ст.преп. Антипова Е..В. 422",
							},
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &denominator,
								Subject:  "Экономическая культура доц. Котар О. К. (4 лек., 4 пр.з.) 241",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     nil,
								Subject:  "пр з. Основы Российской государственности Антипова 410",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     nil,
								Subject:  "Кураторский час",
							},
						},
					},
					{
						Name: parser.Thursday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     nil,
								Subject:  "лек. ОБЩАЯ ЭКОНОМИЧЕСКАЯ ТЕОРИЯ доц. Торопова В.В. 243",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "лек. ОБЩАЯ ЭКОНОМИЧЕСКАЯ ТЕОРИЯ доц. Торопова В.В. 243",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "пр.з. Общая экономическая теория Толстова 314",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     nil,
								Subject:  "пр.з. Математика (базовый уровень) Гиляжева 440",
							},
							{
								TimeFrom: fiveFrom,
								TimeTo:   fiveTo,
								Week:     &numerator,
								Subject:  "пр.з. Русский язык Веселкова 452",
							},
							{
								TimeFrom: fiveFrom,
								TimeTo:   fiveTo,
								Week:     &denominator,
								Subject:  "лек. МАТЕМАТИКА (базовый уровень) доц. Гиляжева Д.Н. 410",
							},
						},
					},
					{
						Name: parser.Friday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     nil,
								Subject:  "пр.з .Иностанный язык Раздобарова, Бормосова 452, 528",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "лек. ОРГ.ТЕХНОЛ. ПРОЦЕССА проф.Забелина М.В, доц. Шьюрова Н.А., Черненко Е.В. 135",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &denominator,
								Subject:  "пр.з.Организация технологическ. процесса Забелина, Шьюрова, Черненко 135",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     nil,
								Subject:  "пр.з. Общая экономическая теория Толстова 232",
							},
						},
					},
				},
			},
		}, {
			Name: "Б-БИ-101",
			Schedule: resource.Schedule{
				Days: []resource.Day{
					{
						Name: parser.Monday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "лек. ФИЗИЧЕСКАЯ КУЛЬТУРА И СПОРТ ФИЗИЧЕСКАЯ КУЛЬТУРА И СПОРТ доц. Кузьмин А.М. 422",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &denominator,
								Subject:  "пр.з. Физическая культура и сорт Фролова 133",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "лек. ИСТОРИЯ РОССИИ ИСТОРИЯ РОССИИ проф Шалаева Н.В. 422",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "лек. ЭКОНОМИЧЕСКАЯ КУЛЬТУРА доц. Аукина И.Г., Аукина (пр.з.) 242",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     &numerator,
								Subject:  "пр з. Основы российской государственности Антипова 314",
							},
						},
					},
					{
						Name: parser.Tuesday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &numerator,
								Subject:  "пр.з. Теория систем и системный анализ Косарев 245",
							},
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &denominator,
								Subject:  "пр.з. Иностранный язык Балашова 526",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     nil,
								Subject:  "пр.з. Иностранный язык Балашова 528",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "лек. ТЕОРИЯ СИСТЕМ И СИСТЕМНЫЙ АНАЛИЗ доц. Косарев А.А. 134 к",
							},
						},
					},
					{
						Name: parser.Wednesday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &numerator,
								Subject:  "лек. ОСНОВЫ РОССИЙСКОЙ ГОСУДАРСТВЕННОСТИ ст.преп. Антипова Е..В. 422",
							},
							{
								TimeFrom: firstFrom,
								TimeTo:   firstTo,
								Week:     &denominator,
								Subject:  "пр з. Основы российской государственности Антипова 422",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     nil,
								Subject:  "пр. з. История России Шалаева 526",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "лек. МИКРОЭКОНОМИКА доц. Васильева О.А. 440",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "пр.з. Микроэкономика Васильева 905",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     nil,
								Subject:  "Кураторский час",
							},
						},
					},
					{
						Name: parser.Thursday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "лек. ВЫСШАЯ МАТЕМАТИКА доц. Гиляжева Д.Н. 422",
							},
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &denominator,
								Subject:  "ЭКОНОМИЧЕСКАЯ КУЛЬТУРА доц. Котар О.К. (лек. и пр.) 422",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &numerator,
								Subject:  "пр.з. Высшая математика Гиляжева 422",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     &denominator,
								Subject:  "пр.з. Теория систем и системный анализ Косарев 245",
							},
						},
					},
					{
						Name: parser.Friday,
						Lessons: []resource.Lesson{
							{
								TimeFrom: secondFrom,
								TimeTo:   secondTo,
								Week:     &numerator,
								Subject:  "пр.з. Высшая математика Гиляжева 510",
							},
							{
								TimeFrom: thirdFrom,
								TimeTo:   thirdTo,
								Week:     nil,
								Subject:  "пр.з. Русский язык Веселкова 452",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     &numerator,
								Subject:  "лек.СПЕЦИАЛЬНАЯ ПЕДАГОГИКА И СПЕЦ.ПСИХОЛОГИЯ доц.Измайлова Ю.М. 341",
							},
							{
								TimeFrom: fourFrom,
								TimeTo:   fourTo,
								Week:     &denominator,
								Subject:  "пр.з. Специальная педагогика и специальная психология Измайлова 341",
							},
						},
					},
				},
			},
		},
	}

	p := parser.NewPDFParser(pdfToCsvScriptPath)

	groups, err := p.ParsePDF(context.Background(), validTestFilePdf)
	if err != nil {
		t.Fatalf("ParsePDF вернул ошибку: %v", err)
	}
	if len(groups) != len(expectedGroups) {
		t.Fatalf("ожидалось %d групп, получено %d", len(expectedGroups), len(groups))
	}

	for i, g := range groups {
		exp := expectedGroups[i]
		if g.Name != exp.Name {
			t.Errorf("ожидалось имя группы %q, получено %q", exp.Name, g.Name)
		}
		if len(g.Schedule.Days) != len(exp.Schedule.Days) {
			t.Errorf("ожидалось %d дней, получено %d", len(exp.Schedule.Days), len(g.Schedule.Days))
			continue
		}
		for di, day := range g.Schedule.Days {
			expDay := exp.Schedule.Days[di]
			if day.Name != expDay.Name {
				t.Errorf("ожидалось имя дня %q, получено %q", expDay.Name, day.Name)
			}
			if len(day.Lessons) != len(expDay.Lessons) {
				t.Errorf("ожидалось %d занятий, получено %d", len(expDay.Lessons), len(day.Lessons))
				continue
			}
			for li, lesson := range day.Lessons {
				expLesson := expDay.Lessons[li]
				if lesson.TimeFrom != expLesson.TimeFrom || lesson.TimeTo != expLesson.TimeTo {
					t.Errorf("ожидалось время %s-%s, получено %s-%s", expLesson.TimeFrom, expLesson.TimeTo, lesson.TimeFrom, lesson.TimeTo)
				}
				if lesson.Subject != expLesson.Subject {
					t.Errorf("ожидалось предмет %q, получено %q", expLesson.Subject, lesson.Subject)
				}
				if !weekValueEqual(lesson.Week, expLesson.Week) {
					t.Errorf("ожидалось week %v, получено %v", expLesson.Week, lesson.Week)
				}
			}
		}
	}
}

func TestPDFParser_ParsePDF_EmptyFile(t *testing.T) {
	p := parser.NewPDFParser(pdfToCsvScriptPath)

	_, err := p.ParsePDF(context.Background(), "fake.pdf")
	if err == nil {
		t.Errorf("ожидалась ошибка при пустом файле, но её не было")
	}
}

func TestPDFParser_ParsePDF_CanceledContext(t *testing.T) {
	p := parser.NewPDFParser(pdfToCsvScriptPath)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // сразу отменяем

	_, err := p.ParsePDF(ctx, validTestFilePdf)
	if err == nil {
		t.Fatalf("ожидалась ошибка при отмене контекста, но её не было")
	}
	t.Logf("Получено: %v", err)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ожидалась ошибка context.Canceled, получено: %v", err)
	}
}

// weekValueEqual проверяет, равны ли значения двух указателей на WeekType.
func weekValueEqual(a, b *resource.WeekType) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
