#!/usr/bin/env sh

DB_SOURCE="/db/downtime.db.sqlite"
DB_TEMP=$(mktemp)

podecho () {
        echo "    POD > " $@
}

podecho "Space in /tmp"
podecho $(df -h | grep "Avail")
podecho $(df -h | grep "/tmp")

podecho "Copying database to temp location..."
cp "$DB_SOURCE" "$DB_TEMP"

SIZE_BEFORE=$(du -hs "$DB_TEMP" | cut -f1)

SQL_CLEAN_BEFORE="DELETE FROM downtime30s  WHERE timeslot < ( SELECT min(timeslot) FROM ( SELECT max(timeslot) as timeslot FROM downtime5m  GROUP BY group_name, probe_name ) );"
SQL_CLEAN_AFTER=" DELETE FROM episodes_30s WHERE timeslot < ( SELECT min(timeslot) FROM ( SELECT max(timeslot) as timeslot FROM episodes_5m GROUP BY group_name, probe_name ) );"
SQL_CLEAN=$(sqlite3 "$DB_TEMP" ".schema" | grep -q "nano" &&
        echo "$SQL_CLEAN_AFTER" ||
        echo "$SQL_CLEAN_BEFORE")

podecho "Cleaning oudated data to reduce the DB size..."
sqlite3 "$DB_TEMP" "$SQL_CLEAN"

podecho "Shrinking free DB space..."
sqlite3 "$DB_TEMP" "PRAGMA auto_vacuum=FULL; VACUUM;"

VER_RECORD=$(sqlite3 "$DB_TEMP" "select * from schema_migrations;")
LAST_MIGRATION_NUMBER=$(echo $VER_RECORD | cut -d'|' -f1)
IS_DIRTY=$(echo $VER_RECORD | cut -d'|' -f2)
if [[ $IS_DIRTY -eq 1 ]]; then
        podecho "Fixing dirty migration table..."
        let "LAST_MIGRATION_NUMBER-=1"
        sqlite3 "$DB_TEMP" "UPDATE schema_migrations SET version=$LAST_MIGRATION_NUMBER, dirty=0;"
fi

SIZE_AFTER=$(du -hs "$DB_TEMP" | cut -f1)
podecho "SIZE BEFORE $SIZE_BEFORE"
podecho "SIZE AFTER  $SIZE_AFTER"

podecho "Restoring the database file..."
cp "$DB_TEMP" "$DB_SOURCE"

podecho "Cleaning temp DB file..."
rm "$DB_TEMP"

podecho "Done. Exiting pod."
exit
