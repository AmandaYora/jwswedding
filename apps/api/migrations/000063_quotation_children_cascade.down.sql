ALTER TABLE quotation_adjustments DROP FOREIGN KEY fk_quotation_adjustments_quotation;
ALTER TABLE quotation_adjustments
  ADD CONSTRAINT fk_quotation_adjustments_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id);

ALTER TABLE quotation_blocks DROP FOREIGN KEY fk_quotation_blocks_quotation;
ALTER TABLE quotation_blocks
  ADD CONSTRAINT fk_quotation_blocks_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id);
