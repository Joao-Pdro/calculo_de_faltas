package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Materia struct {
	ID        int
	Nome      string
	Professor string
}

type Aula struct {
	ID        int
	Data      time.Time
	DiaSemana string
}

type HorarioAula struct {
	ID         int
	AulaID     int
	MateriaID  int
	TipoAula   string
	Inicio     string
	DuracaoMin int
}

type Falta struct {
	ID               int
	HorarioAulaID    int
	Data             time.Time
	HorariosPerdidos int
}

type FaltaResumo struct {
	MateriaNome   string
	Professor     string
	TotalHorarios int
	TotalFaltas   int
	Percentual    float64
	Reprovado     bool
}

type HorarioView struct {
	ID          int
	AulaID      int
	AulaData    string
	MateriaNome string
	Professor   string
	TipoAula    string
	Inicio      string
	Fim         string
	DuracaoMin  int
}

type AulaAgenda struct {
	AulaID     int
	Data       string
	DiaSemana  string
	Horarios   []HorarioView
	SemHorario bool
}

type PageData struct {
	Mensagem string
	Materias []Materia
	Aulas    []Aula
	Horarios []HorarioView
	Agenda   []AulaAgenda
	Resumo   []FaltaResumo
}

type Store struct {
	mu sync.Mutex

	materias []Materia
	aulas    []Aula
	horarios []HorarioAula
	faltas   []Falta

	nextMateriaID int
	nextAulaID    int
	nextHorarioID int
	nextFaltaID   int
}

func NewStore() *Store {
	s := &Store{nextMateriaID: 1, nextAulaID: 1, nextHorarioID: 1, nextFaltaID: 1}
	s.seedCalendarioPadrao()
	return s
}

func (s *Store) seedCalendarioPadrao() {
	inicioSemana := inicioDaSemana(time.Now())
	for i := 0; i < 5; i++ {
		s.addAula(inicioSemana.AddDate(0, 0, i))
	}
}

func inicioDaSemana(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	delta := wd - 1
	base := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return base.AddDate(0, 0, -delta)
}

func diaSemanaPT(t time.Time) string {
	dias := []string{"Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"}
	return dias[t.Weekday()]
}

func parseHHMM(v string) (int, bool) {
	if len(v) != 5 || v[2] != ':' {
		return 0, false
	}
	h, errH := strconv.Atoi(v[:2])
	m, errM := strconv.Atoi(v[3:])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func formatHHMM(totalMin int) string {
	h := (totalMin / 60) % 24
	m := totalMin % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

func (s *Store) addMateria(nome, professor string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.materias = append(s.materias, Materia{ID: s.nextMateriaID, Nome: strings.TrimSpace(nome), Professor: strings.TrimSpace(professor)})
	s.nextMateriaID++
}

func (s *Store) addAula(data time.Time) {
	s.aulas = append(s.aulas, Aula{ID: s.nextAulaID, Data: data, DiaSemana: diaSemanaPT(data)})
	s.nextAulaID++
}

func (s *Store) criarRecorrencia(dataInicio time.Time, diaSemana, semanas int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if semanas < 1 {
		semanas = 1
	}
	delta := (diaSemana - int(dataInicio.Weekday()) + 7) % 7
	primeira := time.Date(dataInicio.Year(), dataInicio.Month(), dataInicio.Day(), 0, 0, 0, 0, dataInicio.Location()).AddDate(0, 0, delta)
	criadas := 0
	for i := 0; i < semanas; i++ {
		dia := primeira.AddDate(0, 0, i*7)
		if s.aulaPorData(dia) != nil {
			continue
		}
		s.addAula(dia)
		criadas++
	}
	sort.Slice(s.aulas, func(i, j int) bool { return s.aulas[i].Data.Before(s.aulas[j].Data) })
	return criadas
}

func (s *Store) aulaPorData(data time.Time) *Aula {
	for i := range s.aulas {
		a := &s.aulas[i]
		if a.Data.Format("2006-01-02") == data.Format("2006-01-02") {
			return a
		}
	}
	return nil
}

func (s *Store) addHorario(aulaID, materiaID int, tipo, inicio string, duracaoMin int) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findAula(aulaID) == nil || s.findMateria(materiaID) == nil {
		return false, "Aula ou matéria inválida"
	}
	if tipo != "teorica" && tipo != "pratica" {
		tipo = "teorica"
	}
	if duracaoMin <= 0 {
		duracaoMin = 50
	}
	inicioMin, okI := parseHHMM(inicio)
	if !okI {
		return false, "Horário inválido: informe início válido"
	}
	fimMin := inicioMin + duracaoMin
	for _, h := range s.horarios {
		if h.AulaID != aulaID {
			continue
		}
		i2, _ := parseHHMM(h.Inicio)
		f2 := i2 + h.DuracaoMin
		if inicioMin < f2 && fimMin > i2 {
			return false, "Conflito: já existe aula nesse intervalo"
		}
	}
	s.horarios = append(s.horarios, HorarioAula{
		ID: s.nextHorarioID, AulaID: aulaID, MateriaID: materiaID, TipoAula: tipo, Inicio: inicio, DuracaoMin: duracaoMin,
	})
	s.nextHorarioID++
	return true, "Horário adicionado"
}

func (s *Store) addFalta(horarioID int, data time.Time, perdidos int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if perdidos < 1 || s.findHorario(horarioID) == nil {
		return false
	}
	s.faltas = append(s.faltas, Falta{ID: s.nextFaltaID, HorarioAulaID: horarioID, Data: data, HorariosPerdidos: perdidos})
	s.nextFaltaID++
	return true
}

func (s *Store) findAula(id int) *Aula {
	for i := range s.aulas {
		if s.aulas[i].ID == id {
			return &s.aulas[i]
		}
	}
	return nil
}

func (s *Store) findMateria(id int) *Materia {
	for i := range s.materias {
		if s.materias[i].ID == id {
			return &s.materias[i]
		}
	}
	return nil
}

func (s *Store) findHorario(id int) *HorarioAula {
	for i := range s.horarios {
		if s.horarios[i].ID == id {
			return &s.horarios[i]
		}
	}
	return nil
}

func (s *Store) pageData(msg string) PageData {
	s.mu.Lock()
	defer s.mu.Unlock()
	materias := append([]Materia(nil), s.materias...)
	aulas := append([]Aula(nil), s.aulas...)
	views := make([]HorarioView, 0, len(s.horarios))
	for _, h := range s.horarios {
		m := s.findMateria(h.MateriaID)
		a := s.findAula(h.AulaID)
		if m == nil || a == nil {
			continue
		}
		inicioMin, _ := parseHHMM(h.Inicio)
		fim := formatHHMM(inicioMin + h.DuracaoMin)
		views = append(views, HorarioView{
			ID: h.ID, AulaID: h.AulaID, AulaData: a.Data.Format("2006-01-02"),
			MateriaNome: m.Nome, Professor: m.Professor, TipoAula: h.TipoAula,
			Inicio: h.Inicio, Fim: fim, DuracaoMin: h.DuracaoMin,
		})
	}
	sort.Slice(materias, func(i, j int) bool {
		if materias[i].Nome == materias[j].Nome {
			return materias[i].Professor < materias[j].Professor
		}
		return materias[i].Nome < materias[j].Nome
	})
	sort.Slice(views, func(i, j int) bool {
		if views[i].AulaData == views[j].AulaData {
			return views[i].Inicio < views[j].Inicio
		}
		return views[i].AulaData < views[j].AulaData
	})

	agendaMap := map[int][]HorarioView{}
	for _, v := range views {
		agendaMap[v.AulaID] = append(agendaMap[v.AulaID], v)
	}
	agenda := make([]AulaAgenda, 0, len(aulas))
	for _, aula := range aulas {
		items := agendaMap[aula.ID]
		agenda = append(agenda, AulaAgenda{
			AulaID: aula.ID, Data: aula.Data.Format("2006-01-02"), DiaSemana: aula.DiaSemana, Horarios: items, SemHorario: len(items) == 0,
		})
	}

	resumo := s.calcularResumoLocked()
	return PageData{Mensagem: msg, Materias: materias, Aulas: aulas, Horarios: views, Agenda: agenda, Resumo: resumo}
}

func (s *Store) calcularResumoLocked() []FaltaResumo {
	res := make([]FaltaResumo, 0, len(s.materias))
	for _, m := range s.materias {
		totalHorarios := 0
		totalFaltas := 0
		for _, h := range s.horarios {
			if h.MateriaID != m.ID {
				continue
			}
			totalHorarios++
			for _, f := range s.faltas {
				if f.HorarioAulaID == h.ID {
					totalFaltas += f.HorariosPerdidos
				}
			}
		}
		percentual := 0.0
		if totalHorarios > 0 {
			percentual = float64(totalFaltas) / float64(totalHorarios) * 100
		}
		res = append(res, FaltaResumo{
			MateriaNome: m.Nome, Professor: m.Professor, TotalHorarios: totalHorarios,
			TotalFaltas: totalFaltas, Percentual: percentual, Reprovado: percentual > 25,
		})
	}
	return res
}

func main() {
	store := NewStore()
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		msg := r.URL.Query().Get("msg")
		if err := tmpl.Execute(w, store.pageData(msg)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/materias", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		nome := r.FormValue("nome")
		prof := r.FormValue("professor")
		msg := "Informe nome e professor"
		if strings.TrimSpace(nome) != "" && strings.TrimSpace(prof) != "" {
			store.addMateria(nome, prof)
			msg = "Matéria criada com sucesso"
		}
		http.Redirect(w, r, "/?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	})

	mux.HandleFunc("/aulas/recorrencia", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		dataInicio, err := time.Parse("2006-01-02", r.FormValue("data_inicio"))
		if err != nil {
			http.Redirect(w, r, "/?msg="+url.QueryEscape("Data inválida"), http.StatusSeeOther)
			return
		}
		dia, _ := strconv.Atoi(r.FormValue("dia_semana"))
		semanas, _ := strconv.Atoi(r.FormValue("semanas"))
		criadas := store.criarRecorrencia(dataInicio, dia, semanas)
		http.Redirect(w, r, "/?msg="+url.QueryEscape(fmt.Sprintf("Recorrência aplicada: %d dia(s) criado(s)", criadas)), http.StatusSeeOther)
	})

	mux.HandleFunc("/horarios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		aulaID, _ := strconv.Atoi(r.FormValue("aula_id"))
		materiaID, _ := strconv.Atoi(r.FormValue("materia_id"))
		duracaoMin, _ := strconv.Atoi(r.FormValue("duracao_min"))
		ok, msg := store.addHorario(aulaID, materiaID, r.FormValue("tipo_aula"), r.FormValue("inicio"), duracaoMin)
		if !ok && msg == "" {
			msg = "Erro ao adicionar horário"
		}
		http.Redirect(w, r, "/?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	})

	mux.HandleFunc("/faltas", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		horarioID, _ := strconv.Atoi(r.FormValue("horario_id"))
		perdidos, _ := strconv.Atoi(r.FormValue("horarios_perdidos"))
		dataFalta, err := time.Parse("2006-01-02", r.FormValue("data_falta"))
		if err != nil {
			http.Redirect(w, r, "/?msg="+url.QueryEscape("Data de falta inválida"), http.StatusSeeOther)
			return
		}
		ok := store.addFalta(horarioID, dataFalta, perdidos)
		msg := "Erro ao registrar falta"
		if ok {
			msg = "Falta registrada"
		}
		http.Redirect(w, r, "/?msg="+url.QueryEscape(msg), http.StatusSeeOther)
	})

	log.Println("Servidor iniciado em http://localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", mux))
}
