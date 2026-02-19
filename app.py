from __future__ import annotations

import html
import sqlite3
from contextlib import closing
from datetime import date, datetime, timedelta
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse

BASE_DIR = Path(__file__).resolve().parent
DB_PATH = BASE_DIR / "database.sqlite3"


STYLE = """
:root { font-family: Arial, sans-serif; }
body { margin: 0; background: #f4f6fb; }
main { max-width: 1000px; margin: 0 auto; padding: 1.5rem; }
section { background: #fff; margin: 1rem 0; padding: 1rem; border-radius: 8px; }
.grid-form { display: grid; grid-template-columns: repeat(auto-fit,minmax(200px,1fr)); gap: .8rem; align-items: end; }
label { display:flex; flex-direction:column; gap:.3rem; }
button { background:#2f65f5; color:#fff; border:none; border-radius:6px; padding:.6rem; cursor:pointer; }
.flash { background:#e8f7e8; border:1px solid #4caf50; padding:.5rem; border-radius:4px; }
table { width:100%; border-collapse: collapse; }
th,td { border:1px solid #ddd; padding:.4rem; }
.ok { color:#0b7c28; font-weight:bold; }
.reprovado { color:#a60d0d; font-weight:bold; }
"""


def get_db() -> sqlite3.Connection:
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db() -> None:
    with closing(get_db()) as conn:
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS materias (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                nome TEXT NOT NULL,
                professor TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS aulas (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                data TEXT NOT NULL,
                dia_semana TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS horarios_aula (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                aula_id INTEGER NOT NULL,
                materia_id INTEGER NOT NULL,
                tipo_aula TEXT NOT NULL CHECK(tipo_aula IN ('teorica', 'pratica')),
                inicio TEXT NOT NULL,
                FOREIGN KEY (aula_id) REFERENCES aulas(id),
                FOREIGN KEY (materia_id) REFERENCES materias(id)
            );
            CREATE TABLE IF NOT EXISTS faltas (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                horario_id INTEGER NOT NULL,
                data_falta TEXT NOT NULL,
                horarios_perdidos INTEGER NOT NULL CHECK(horarios_perdidos > 0),
                FOREIGN KEY (horario_id) REFERENCES horarios_aula(id)
            );
            """
        )
        conn.commit()


def seed_default_calendar() -> None:
    with closing(get_db()) as conn:
        total = conn.execute("SELECT COUNT(*) total FROM aulas").fetchone()["total"]
        if total:
            return
        hoje = date.today()
        monday = hoje - timedelta(days=hoje.weekday())
        for i in range(5):
            dia = monday + timedelta(days=i)
            conn.execute(
                "INSERT INTO aulas (data, dia_semana) VALUES (?, ?)",
                (dia.isoformat(), dia.strftime("%A")),
            )
        conn.commit()


def calcular_faltas():
    with closing(get_db()) as conn:
        rows = conn.execute(
            """
            SELECT m.nome materia_nome, m.professor professor,
                   COUNT(h.id) total_horarios,
                   COALESCE(SUM(f.horarios_perdidos),0) total_faltas
            FROM materias m
            LEFT JOIN horarios_aula h ON h.materia_id = m.id
            LEFT JOIN faltas f ON f.horario_id = h.id
            GROUP BY m.id
            ORDER BY m.nome, m.professor
            """
        ).fetchall()

    result = []
    for row in rows:
        total_horarios = int(row["total_horarios"])
        total_faltas = int(row["total_faltas"])
        percentual = (total_faltas / total_horarios * 100.0) if total_horarios else 0.0
        result.append(
            {
                "materia_nome": row["materia_nome"],
                "professor": row["professor"],
                "total_horarios": total_horarios,
                "total_faltas": total_faltas,
                "percentual": percentual,
                "reprovado": percentual > 25,
            }
        )
    return result


def options_html(options, value_key, text_builder, placeholder="Selecione"):
    html_parts = [f'<option value="">{placeholder}</option>']
    for item in options:
        val = item[value_key]
        text = text_builder(item)
        html_parts.append(f'<option value="{val}">{html.escape(text)}</option>')
    return "".join(html_parts)


def render_page(message: str = "") -> str:
    with closing(get_db()) as conn:
        materias = conn.execute("SELECT * FROM materias ORDER BY nome, professor").fetchall()
        aulas = conn.execute("SELECT * FROM aulas ORDER BY data").fetchall()
        horarios = conn.execute(
            """
            SELECT h.id, h.tipo_aula, h.inicio, m.nome materia_nome, m.professor
            FROM horarios_aula h
            JOIN materias m ON m.id = h.materia_id
            ORDER BY h.inicio
            """
        ).fetchall()

    resumo = calcular_faltas()
    msg_html = f'<p class="flash">{html.escape(message)}</p>' if message else ""
    rows = "".join(
        f"""
        <tr>
          <td>{html.escape(x['materia_nome'])}</td>
          <td>{html.escape(x['professor'])}</td>
          <td>{x['total_horarios']}</td>
          <td>{x['total_faltas']}</td>
          <td>{x['percentual']:.1f}%</td>
          <td class={'reprovado' if x['reprovado'] else 'ok'}>{'BOMBA (>25%)' if x['reprovado'] else 'Dentro do limite'}</td>
        </tr>
        """
        for x in resumo
    )

    return f"""
<!doctype html>
<html lang='pt-BR'>
<head><meta charset='UTF-8'><title>Controle de Faltas</title><style>{STYLE}</style></head>
<body><main>
<h1>Controle de Faltas</h1>
{msg_html}
<section><h2>1) Cadastrar Matéria</h2>
<form method='post' action='/materias' class='grid-form'>
<label>Nome da matéria<input name='nome' required></label>
<label>Professor<input name='professor' required></label>
<button>Salvar matéria</button>
</form></section>

<section><h2>2) Calendário de Aulas (recorrência)</h2>
<form method='post' action='/aulas/recorrencia' class='grid-form'>
<label>Data de início<input type='date' name='data_inicio' required></label>
<label>Dia da semana<select name='dia_semana'>
<option value='0'>Segunda</option><option value='1'>Terça</option><option value='2'>Quarta</option>
<option value='3'>Quinta</option><option value='4'>Sexta</option><option value='5'>Sábado</option><option value='6'>Domingo</option>
</select></label>
<label>Quantidade de semanas<input type='number' min='1' name='semanas' value='1'></label>
<button>Aplicar recorrência</button>
</form></section>

<section><h2>3) Cadastrar horário em um dia de aula</h2>
<form method='post' action='/horarios' class='grid-form'>
<label>Dia de aula<select name='aula_id' required>{options_html(aulas,'id',lambda a: f"{a['data']} ({a['dia_semana']})")}</select></label>
<label>Matéria<select name='materia_id' required>{options_html(materias,'id',lambda m: f"{m['nome']} - {m['professor']}")}</select></label>
<label>Tipo<select name='tipo_aula'><option value='teorica'>Teórica</option><option value='pratica'>Prática</option></select></label>
<label>Horário de início<input type='time' name='inicio' required></label>
<button>Adicionar horário</button>
</form></section>

<section><h2>4) Registrar falta</h2>
<form method='post' action='/faltas' class='grid-form'>
<label>Horário<select name='horario_id' required>{options_html(horarios,'id',lambda h: f"{h['materia_nome']} - {h['professor']} ({h['inicio']}, {h['tipo_aula']})")}</select></label>
<label>Data da falta<input type='date' name='data_falta' required></label>
<label>Horários perdidos<input type='number' min='1' name='horarios_perdidos' value='1'></label>
<button>Salvar falta</button>
</form></section>

<section><h2>Resumo de faltas por matéria</h2>
<table><thead><tr><th>Matéria</th><th>Professor</th><th>Horários</th><th>Faltas</th><th>%</th><th>Status</th></tr></thead><tbody>{rows}</tbody></table>
</section>
</main></body></html>
"""


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path != "/":
            self.send_error(HTTPStatus.NOT_FOUND, "Página não encontrada")
            return
        message = parse_qs(parsed.query).get("msg", [""])[0]
        content = render_page(message)
        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(content.encode("utf-8"))

    def do_POST(self):
        parsed = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0"))
        data = parse_qs(self.rfile.read(length).decode("utf-8"))

        def field(name: str, default: str = "") -> str:
            return data.get(name, [default])[0]

        message = "Operação concluída"
        with closing(get_db()) as conn:
            if parsed.path == "/materias":
                nome, professor = field("nome"), field("professor")
                if nome and professor:
                    conn.execute("INSERT INTO materias (nome, professor) VALUES (?, ?)", (nome.strip(), professor.strip()))
                    conn.commit()
                    message = "Matéria criada com sucesso"
                else:
                    message = "Informe nome e professor"

            elif parsed.path == "/aulas/recorrencia":
                try:
                    data_inicio = datetime.strptime(field("data_inicio"), "%Y-%m-%d").date()
                    dia_semana = int(field("dia_semana", "0"))
                    semanas = max(1, int(field("semanas", "1")))
                    delta = (dia_semana - data_inicio.weekday()) % 7
                    primeira = data_inicio + timedelta(days=delta)
                    criadas = 0
                    for i in range(semanas):
                        dia = primeira + timedelta(weeks=i)
                        exists = conn.execute("SELECT id FROM aulas WHERE data=?", (dia.isoformat(),)).fetchone()
                        if not exists:
                            conn.execute(
                                "INSERT INTO aulas (data, dia_semana) VALUES (?, ?)",
                                (dia.isoformat(), dia.strftime("%A")),
                            )
                            criadas += 1
                    conn.commit()
                    message = f"Recorrência aplicada: {criadas} dia(s) criado(s)"
                except ValueError:
                    message = "Dados de recorrência inválidos"

            elif parsed.path == "/horarios":
                try:
                    conn.execute(
                        "INSERT INTO horarios_aula (aula_id, materia_id, tipo_aula, inicio) VALUES (?, ?, ?, ?)",
                        (int(field("aula_id")), int(field("materia_id")), field("tipo_aula", "teorica"), field("inicio")),
                    )
                    conn.commit()
                    message = "Horário adicionado"
                except (ValueError, sqlite3.DatabaseError):
                    message = "Erro ao adicionar horário"

            elif parsed.path == "/faltas":
                try:
                    conn.execute(
                        "INSERT INTO faltas (horario_id, data_falta, horarios_perdidos) VALUES (?, ?, ?)",
                        (int(field("horario_id")), field("data_falta"), max(1, int(field("horarios_perdidos", "1")))),
                    )
                    conn.commit()
                    message = "Falta registrada"
                except (ValueError, sqlite3.DatabaseError):
                    message = "Erro ao registrar falta"

            else:
                self.send_error(HTTPStatus.NOT_FOUND, "Rota inexistente")
                return

        self.send_response(HTTPStatus.SEE_OTHER)
        self.send_header("Location", f"/?msg={message.replace(' ', '+')}")
        self.end_headers()


if __name__ == "__main__":
    init_db()
    seed_default_calendar()
    server = ThreadingHTTPServer(("0.0.0.0", 8000), Handler)
    print("Servidor iniciado em http://localhost:8000")
    server.serve_forever()
