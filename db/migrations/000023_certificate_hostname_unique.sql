DELETE FROM certificates a
WHERE a.id IN (
    SELECT id FROM (
        SELECT id, row_number() OVER (
            PARTITION BY account_id, hostname
            ORDER BY
                CASE status
                    WHEN 'active' THEN 0
                    WHEN 'renewing' THEN 1
                    WHEN 'requested' THEN 2
                    ELSE 3
                END,
                not_after DESC NULLS LAST,
                created_at DESC
        ) AS rn
        FROM certificates
    ) ranked
    WHERE ranked.rn > 1
);

CREATE UNIQUE INDEX certificates_account_hostname_uidx
    ON certificates (account_id, hostname);
