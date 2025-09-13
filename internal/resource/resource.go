package resource

// WeekType — тип недели для занятия.
// nil (указатель == nil) означает: занятие проходит и по числителю, и по знаменателю (т.е. "оба").
// В отличии от пустой строки, nil задаёт именно отсутствие ограничения по типу недели.
type WeekType string

const (
	WeekNumerator   WeekType = "numerator"   // числитель
	WeekDenominator WeekType = "denominator" // знаменатель
	// nil — означает "и числитель, и знаменатель" (т.е. нет ограничения)
)

// Lesson — один элемент массива занятий в день.
// TimeFrom/TimeTo — время в формате "15:04" (чтобы удобно сериализовать/читать).
// Subject — название предмета/лекции/семинара.
// Lecturer — необязательное поле — кто ведёт (можно оставить пустым).
// Room — аудитория/кабинет (опционально).
// Week — указатель на WeekType. nil означает "занятие по обоим неделям".
type Lesson struct {
	TimeFrom string `json:"time_from"` // начало в формате "HH:MM"
	TimeTo   string `json:"time_to"`   // конец в формате "HH:MM"
	Subject  string `json:"subject"`   // название предмета
	// Lecturer string    `json:"lecturer,omitempty"` // преподаватель
	// Room     string    `json:"room,omitempty"`     // аудитория
	Week *WeekType `json:"week,omitempty"` // nil => оба (числитель+знаменатель)
}

// Day — день недели как подструктура расписания.
// Name — "Monday", "Tuesday" или на русском "Понедельник" и т.д.
// Lessons — массив занятий в этот день.
type Day struct {
	Name    string   `json:"name"`              // название дня (лучше хранить в одном языке)
	Lessons []Lesson `json:"lessons,omitempty"` // занятия в этот дне
}

// Schedule — расписание для одной группы (может быть на семестр/неделю).
// PeriodStart/PeriodEnd — период действия расписания (опционально).
// Days — подструктуры (дни недели). Можно хранить как массив (Mon..Sun) или только дни с занятиями.
type Schedule struct {
	// PeriodStart *time.Time `json:"period_start,omitempty"` // начало периода расписания (опционально)
	// PeriodEnd   *time.Time `json:"period_end,omitempty"`   // конец периода (опционально)
	Days []Day `json:"days,omitempty"` // дни недели (подструктуры)
}

// Group — группа.
// Name — номер/название группы.
// Course — курс/год обучения (1,2,3...).
// Schedule — массив или одиночное расписание. Мы используем один Schedule, но можно расширить до []Schedule.
type Group struct {
	Name string `json:"name"` // название/номер группы
	// Course   int               `json:"course,omitempty"`   // курс
	Schedule Schedule `json:"schedule,omitempty"` // расписание группы
}

// StudyForm — форма обучения (очная, заочная, очно-заочная).
// Key — ключ формы, например "och" или "zao".
// DisplayName — удобочитаемое имя.
type StudyForm struct {
	// Key    string  `json:"key"`              // внутренний ключ формы обучения
	Name   string  `json:"name"`             // отображаемое имя формы обучения
	Groups []Group `json:"groups,omitempty"` // группы для данной формы
}

// Institute — институт/факультет.
// Code — код института.
// Name — название института.
// Forms — формы обучения внутри института (о/зао).
type Institute struct {
	// Code  string            `json:"code"`            // код института (например "INF")
	Name  string      `json:"name"`            // полное название института
	Forms []StudyForm `json:"forms,omitempty"` // формы обучения внутри института
}

// Building — учебный корпус.
// Institutes — список институтов в корпусе.
type Building struct {
	Name       string      `json:"name"`                 // название корпуса
	Institutes []Institute `json:"institutes,omitempty"` // институты в корпусе
}
