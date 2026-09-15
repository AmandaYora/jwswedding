ALTER TABLE client_contacts RENAME INDEX idx_client_contacts_project TO idx_clients_project;
ALTER TABLE client_contacts RENAME INDEX idx_client_contacts_tenant TO idx_clients_tenant;

RENAME TABLE client_contacts TO clients;
