ALTER TABLE projects DROP INDEX idx_projects_client;
ALTER TABLE projects DROP COLUMN client_id;

ALTER TABLE client_contacts DROP FOREIGN KEY fk_client_contacts_client;
ALTER TABLE client_contacts DROP INDEX idx_client_contacts_client;
ALTER TABLE client_contacts DROP COLUMN client_id;

DROP TABLE clients;
