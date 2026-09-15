ALTER TABLE projects DROP INDEX uq_projects_quotation;
ALTER TABLE projects DROP COLUMN quotation_id;

DROP TABLE quotation_adjustments;
DROP TABLE quotation_blocks;
DROP TABLE quotations;
