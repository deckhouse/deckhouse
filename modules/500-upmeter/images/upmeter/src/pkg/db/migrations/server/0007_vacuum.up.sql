/*

This attempts to save disk space by reducing the database file size and reusing cleaned space in it.

*/

PRAGMA auto_vacuum = FULL;
VACUUM;

