# migrations

| Name            | Type         | Key | Comment                        |
|-----------------|--------------|-----|--------------------------------|
| project         | varchar(16)  | PRI | Microservice or project name   |
| filename        | varchar(255) | PRI | yyyy-mm-dd-HHMMSS.sql          |
| statement_index | int(11)      |     | Statement number from SQL file |
| status          | text         |     | ok or full error message       |
