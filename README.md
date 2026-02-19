# Sistema de Controle de Faltas

Aplicação web local para gerenciar matérias, horários de aula e faltas.

## Requisitos atendidos

- Cadastro de várias matérias, incluindo matérias com mesmo nome e professores diferentes.
- Cadastro de horários em um dia de aula.
- Calendário padrão criado automaticamente (semana atual) e possibilidade de recorrência por 1 dia/semana ou várias semanas.
- Cálculo da porcentagem de faltas por matéria com regra de reprovação acima de 25%.

## Rodar localmente

```bash
python app.py
```

Acesse: http://localhost:8000

## Rodar com Docker

```bash
docker compose up --build
```

Acesse: http://localhost:8000
