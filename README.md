# Sistema de Controle de Faltas (Go)

Aplicação web local em Go com frontend em templates (`html/template`).

## Requisitos atendidos

- Cadastro de várias matérias, inclusive com o mesmo nome e professores diferentes.
- Cadastro de horários em um dia de aula, agora com **hora de início e fim**.
- Calendário padrão inicial e recorrência configurável para 1 ou várias semanas.
- Montagem do calendário por dia exibindo todos os intervalos cadastrados.
- Validação de conflito de horário no mesmo dia (não permite sobreposição de intervalos).
- Cálculo de porcentagem de faltas por matéria com reprovação acima de 25%.

## Rodar localmente

```bash
go run main.go
```

Acesse: http://localhost:8000

## Rodar com Docker

```bash
docker compose up --build
```

Acesse: http://localhost:8000
