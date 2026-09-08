CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    event VARCHAR(80) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    channel VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'webhook', 'inbox')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);

CREATE TABLE notification_rules (
    id UUID PRIMARY KEY,
    event VARCHAR(80) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    target TEXT NOT NULL,
    cooldown_seconds INTEGER NOT NULL DEFAULT 300
);
