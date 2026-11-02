-- Reverse 0019: drop the non-overlap guarantee.

ALTER TABLE billing_periods DROP CONSTRAINT IF EXISTS billing_periods_no_overlap;
