# Уникальный список инструментов, отсутствующих в busybox

Источник: `WORKFLOWS_RUN_CLI_DOCKER_MERGED.md`.

Критерии отбора:
- Из основной таблицы: значение в колонке «Docker-образ / статус» **не** равно `часть busybox`.
- Из дополнительных таблиц: значение в колонке «Наличие в busybox» равно `Нет`.
- Дубликаты между разделами удалены.

## Итоговый список (уникальные инструменты)

1. `bash`
2. `check-release-images.sh`
3. `crane`
4. `curl`
5. `dhctl`
6. `docker`
7. `gh`
8. `git`
9. `jq`
10. `make`
11. `npm`
12. `pip`
13. `python`
14. `python3`
15. `regctl`
16. `render-workflows.sh`
17. `rsync`
18. `ssh-keygen`
19. `validate_dictionary_sync.sh`
20. `validate_wordlist.sh`
21. `validation_bashible.sh`
22. `validation_run.sh`
23. `werf`

Всего: **23** инструмента.
