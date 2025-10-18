CREATE TABLE IF NOT EXISTS banner_clicks (
  banner_id BIGINT      NOT NULL,
  ts        TIMESTAMPTZ NOT NULL,  -- начало минуты (UTC)
  cnt       BIGINT      NOT NULL,
  PRIMARY KEY (banner_id, ts)
);

CREATE INDEX IF NOT EXISTS idx_banner_clicks_bid_ts
  ON banner_clicks (banner_id, ts);
