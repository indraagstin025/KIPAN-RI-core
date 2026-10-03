-- ROLLBACK 000025_pengurus_status_efektif

DROP VIEW IF EXISTS v_pengurus_efektif;
DROP FUNCTION IF EXISTS pengurus_status_efektif(TEXT, TEXT, DATE, DATE);
