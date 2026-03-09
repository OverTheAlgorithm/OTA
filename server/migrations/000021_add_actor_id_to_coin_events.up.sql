-- Track who triggered the coin event (NULL = system, e.g. signup bonus)
ALTER TABLE coin_events ADD COLUMN actor_id UUID REFERENCES users(id);
CREATE INDEX idx_coin_events_actor_id ON coin_events(actor_id);
