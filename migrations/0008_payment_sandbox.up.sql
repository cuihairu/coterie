-- Payment Plugins (Phase 3): the sandbox channel registers alongside
-- manual — a deterministic offline adapter for demos and tests, and
-- the template real channels follow. The method CHECK list extends by
-- migration, as the payment ledger's contract states (D11).

ALTER TABLE payments DROP CONSTRAINT payments_method_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_method_check
    CHECK (method IN ('manual', 'sandbox'));
