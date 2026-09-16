-- Filter Sales + Wedding Planner pada daftar Penawaran/Client/Monitoring
-- Timeline (PLAN wording-role-dan-filter-sales-wp §5.1): tiga index baru untuk
-- kolom filter yang selama ini tidak ber-index.
ALTER TABLE quotations ADD KEY idx_quotations_tenant_created_by (tenant_id, created_by_staff_id);
ALTER TABLE projects ADD KEY idx_projects_tenant_pic_sales (tenant_id, pic_sales_staff_id);
ALTER TABLE projects ADD KEY idx_projects_tenant_client (tenant_id, client_id);
