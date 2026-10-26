CREATE TABLE public.app_settings (
    id bigint NOT NULL,
    key character varying NOT NULL,
    note text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    value numeric(16,8) NOT NULL
);
CREATE SEQUENCE public.app_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.app_settings_id_seq OWNED BY public.app_settings.id;
CREATE TABLE public.coins (
    id bigint NOT NULL,
    name character varying NOT NULL,
    description text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    deleted_at timestamp(6) without time zone,
    market_cap bigint
);
CREATE SEQUENCE public.coins_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.coins_id_seq OWNED BY public.coins.id;
CREATE TABLE public.dca_settings (
    id bigint NOT NULL,
    pair_id bigint NOT NULL,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    used_budget numeric(20,8) DEFAULT 0.0 NOT NULL
);
CREATE SEQUENCE public.dca_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.dca_settings_id_seq OWNED BY public.dca_settings.id;
CREATE TABLE public.dca_trades (
    id bigint NOT NULL,
    dca_setting_id bigint NOT NULL,
    open_price numeric(16,8),
    close_price numeric(16,8),
    amount numeric(20,8) DEFAULT 0.0 NOT NULL,
    open_at timestamp(6) without time zone,
    close_at timestamp(6) without time zone,
    realized_profit numeric(16,8) DEFAULT 0.0 NOT NULL,
    unrealized_profit numeric(16,8) DEFAULT 0.0 NOT NULL,
    status character varying DEFAULT 'OPENING'::character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.dca_trades_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.dca_trades_id_seq OWNED BY public.dca_trades.id;
CREATE TABLE public.ema1d_candles (
    id bigint NOT NULL,
    pair_id bigint NOT NULL,
    "interval" character varying NOT NULL,
    open_time timestamp(6) without time zone NOT NULL,
    open double precision NOT NULL,
    high double precision NOT NULL,
    low double precision NOT NULL,
    close double precision NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.ema1d_candles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_candles_id_seq OWNED BY public.ema1d_candles.id;
CREATE TABLE public.ema1d_daily_checks (
    id bigint NOT NULL,
    signal_date date NOT NULL,
    status character varying DEFAULT 'PENDING'::character varying NOT NULL,
    run_id character varying,
    attempts integer DEFAULT 0 NOT NULL,
    rows jsonb DEFAULT '[]'::jsonb NOT NULL,
    sync_params jsonb DEFAULT '{}'::jsonb NOT NULL,
    sync_result jsonb DEFAULT '{}'::jsonb NOT NULL,
    report text,
    last_error text,
    synced_at timestamp(6) without time zone,
    sent_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.ema1d_daily_checks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_daily_checks_id_seq OWNED BY public.ema1d_daily_checks.id;
CREATE TABLE public.ema1d_equity_snapshots (
    id bigint NOT NULL,
    date date NOT NULL,
    equity_usdt numeric(16,8) DEFAULT 0.0 NOT NULL,
    margin_level numeric(16,8),
    open_long_notional numeric(16,8) DEFAULT 0.0 NOT NULL,
    open_short_notional numeric(16,8) DEFAULT 0.0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.ema1d_equity_snapshots_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_equity_snapshots_id_seq OWNED BY public.ema1d_equity_snapshots.id;
CREATE TABLE public.ema1d_pairs (
    id bigint NOT NULL,
    pair_id bigint NOT NULL,
    allow_long boolean DEFAULT true NOT NULL,
    allow_short boolean DEFAULT true NOT NULL,
    short_supported boolean DEFAULT false NOT NULL,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    added_on date NOT NULL,
    removed_on date,
    checked_at timestamp(6) without time zone,
    check_note text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    pair_group character varying DEFAULT 'CORE'::character varying NOT NULL,
    long_exit_rule character varying,
    signal_from date
);
CREATE SEQUENCE public.ema1d_pairs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_pairs_id_seq OWNED BY public.ema1d_pairs.id;
CREATE TABLE public.ema1d_syncs (
    id bigint NOT NULL,
    run_id character varying NOT NULL,
    signal_date date NOT NULL,
    raw_query text,
    parsed jsonb DEFAULT '{}'::jsonb NOT NULL,
    result jsonb DEFAULT '{}'::jsonb NOT NULL,
    dry_run boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.ema1d_syncs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_syncs_id_seq OWNED BY public.ema1d_syncs.id;
CREATE TABLE public.ema1d_todos (
    id bigint NOT NULL,
    ema1d_pair_id bigint NOT NULL,
    action character varying NOT NULL,
    side character varying NOT NULL,
    signal_date date NOT NULL,
    ref_date date,
    ref_price numeric(16,8),
    exit_rule character varying,
    source character varying DEFAULT 'SYNC'::character varying NOT NULL,
    ema1d_sync_id bigint,
    depends_on_id bigint,
    ema_trade_id bigint,
    status character varying DEFAULT 'PENDING'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    run_after timestamp(6) without time zone,
    last_error text,
    step character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    first_attempted_at timestamp(6) without time zone
);
CREATE SEQUENCE public.ema1d_todos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema1d_todos_id_seq OWNED BY public.ema1d_todos.id;
CREATE TABLE public.ema_settings (
    id bigint NOT NULL,
    pair_id bigint NOT NULL,
    budget numeric(16,8) DEFAULT 0.0 NOT NULL,
    trade_type character varying DEFAULT 'BOTH'::character varying NOT NULL,
    period_type character varying DEFAULT 'D1'::character varying NOT NULL,
    check_type character varying DEFAULT 'CHECK_AT_OPEN'::character varying NOT NULL,
    next_trade_type character varying DEFAULT 'NEW_TRADE'::character varying NOT NULL,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    tp_percent numeric(8,4) DEFAULT 0.0 NOT NULL,
    sl_percent numeric(8,4) DEFAULT 0.0 NOT NULL,
    note text
);
CREATE SEQUENCE public.ema_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema_settings_id_seq OWNED BY public.ema_settings.id;
CREATE TABLE public.ema_trades (
    id bigint NOT NULL,
    pair_id bigint NOT NULL,
    open_price numeric(16,8),
    close_price numeric(16,8),
    sl_price numeric(16,8) DEFAULT 0.0 NOT NULL,
    amount numeric(20,8) DEFAULT 0.0 NOT NULL,
    status character varying NOT NULL,
    trade_type character varying DEFAULT ''::character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    is_auto_trade boolean DEFAULT false,
    period_type character varying DEFAULT 'D1'::character varying NOT NULL,
    over_cross_ema_for_open numeric(6,4) DEFAULT 0.0 NOT NULL,
    over_cross_ema_for_close numeric(6,4) DEFAULT 0.0 NOT NULL,
    open_at timestamp(6) without time zone,
    close_at timestamp(6) without time zone,
    strategy character varying DEFAULT 'CHECK_AT_CURRENT'::character varying NOT NULL,
    next_trade_type character varying DEFAULT 'NEW_TRADE'::character varying NOT NULL,
    ema_setting_id bigint,
    realized_profit numeric(16,8) DEFAULT 0.0 NOT NULL,
    unrealized_profit numeric(16,8) DEFAULT 0.0 NOT NULL,
    source character varying DEFAULT 'EMA_AUTO'::character varying NOT NULL,
    signal_date date,
    ref_open_price numeric(16,8),
    ref_close_price numeric(16,8),
    exit_rule character varying,
    size_percent numeric(8,4),
    equity_at_open numeric(16,8),
    notional_usdt numeric(16,8),
    interest_paid numeric(20,8) DEFAULT 0.0 NOT NULL,
    interest_usdt numeric(16,8) DEFAULT 0.0 NOT NULL,
    fee_usdt numeric(16,8) DEFAULT 0.0 NOT NULL
);
CREATE SEQUENCE public.ema_trades_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.ema_trades_id_seq OWNED BY public.ema_trades.id;
CREATE TABLE public.markets (
    id bigint NOT NULL,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);
CREATE SEQUENCE public.markets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.markets_id_seq OWNED BY public.markets.id;
CREATE TABLE public.pairs (
    id bigint NOT NULL,
    coin_id bigint NOT NULL,
    market_id bigint NOT NULL,
    name character varying NOT NULL,
    current_price numeric(16,8),
    interest_rate numeric(6,2),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    is_spot boolean DEFAULT false NOT NULL,
    deleted_at timestamp(6) without time zone,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    is_auto_trade boolean DEFAULT false NOT NULL,
    borrowed_amount numeric(16,8),
    is_highlight boolean DEFAULT false NOT NULL,
    is_isolated boolean DEFAULT false NOT NULL,
    is_auto_ema_h4_trade boolean DEFAULT false NOT NULL,
    is_auto_ema_d1_trade boolean DEFAULT false NOT NULL,
    is_auto_ema_h1_trade boolean DEFAULT false NOT NULL,
    step_size numeric(16,8)
);
CREATE SEQUENCE public.pairs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.pairs_id_seq OWNED BY public.pairs.id;
CREATE TABLE public.users (
    id bigint NOT NULL,
    email character varying NOT NULL,
    uid character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    ips character varying[] DEFAULT '{}'::character varying[]
);
CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;
ALTER TABLE ONLY public.app_settings ALTER COLUMN id SET DEFAULT nextval('public.app_settings_id_seq'::regclass);
ALTER TABLE ONLY public.coins ALTER COLUMN id SET DEFAULT nextval('public.coins_id_seq'::regclass);
ALTER TABLE ONLY public.dca_settings ALTER COLUMN id SET DEFAULT nextval('public.dca_settings_id_seq'::regclass);
ALTER TABLE ONLY public.dca_trades ALTER COLUMN id SET DEFAULT nextval('public.dca_trades_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_candles ALTER COLUMN id SET DEFAULT nextval('public.ema1d_candles_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_daily_checks ALTER COLUMN id SET DEFAULT nextval('public.ema1d_daily_checks_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_equity_snapshots ALTER COLUMN id SET DEFAULT nextval('public.ema1d_equity_snapshots_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_pairs ALTER COLUMN id SET DEFAULT nextval('public.ema1d_pairs_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_syncs ALTER COLUMN id SET DEFAULT nextval('public.ema1d_syncs_id_seq'::regclass);
ALTER TABLE ONLY public.ema1d_todos ALTER COLUMN id SET DEFAULT nextval('public.ema1d_todos_id_seq'::regclass);
ALTER TABLE ONLY public.ema_settings ALTER COLUMN id SET DEFAULT nextval('public.ema_settings_id_seq'::regclass);
ALTER TABLE ONLY public.ema_trades ALTER COLUMN id SET DEFAULT nextval('public.ema_trades_id_seq'::regclass);
ALTER TABLE ONLY public.markets ALTER COLUMN id SET DEFAULT nextval('public.markets_id_seq'::regclass);
ALTER TABLE ONLY public.pairs ALTER COLUMN id SET DEFAULT nextval('public.pairs_id_seq'::regclass);
ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);
ALTER TABLE ONLY public.app_settings
    ADD CONSTRAINT app_settings_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.coins
    ADD CONSTRAINT coins_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.dca_settings
    ADD CONSTRAINT dca_settings_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.dca_trades
    ADD CONSTRAINT dca_trades_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_candles
    ADD CONSTRAINT ema1d_candles_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_daily_checks
    ADD CONSTRAINT ema1d_daily_checks_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_equity_snapshots
    ADD CONSTRAINT ema1d_equity_snapshots_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_pairs
    ADD CONSTRAINT ema1d_pairs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_syncs
    ADD CONSTRAINT ema1d_syncs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema1d_todos
    ADD CONSTRAINT ema1d_todos_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema_settings
    ADD CONSTRAINT ema_settings_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.ema_trades
    ADD CONSTRAINT ema_trades_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.markets
    ADD CONSTRAINT markets_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.pairs
    ADD CONSTRAINT pairs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);
CREATE INDEX coins_on_deleted_at ON public.coins USING btree (deleted_at);
CREATE UNIQUE INDEX coins_on_name ON public.coins USING btree (name);
CREATE INDEX index_dca_settings_on_pair_id ON public.dca_settings USING btree (pair_id);
CREATE INDEX index_dca_settings_on_pair_id_and_status ON public.dca_settings USING btree (pair_id, status);
CREATE INDEX index_dca_trades_on_dca_setting_id ON public.dca_trades USING btree (dca_setting_id);
CREATE INDEX index_dca_trades_on_dca_setting_id_and_status ON public.dca_trades USING btree (dca_setting_id, status);
CREATE UNIQUE INDEX index_ema1d_candles_uniqueness ON public.ema1d_candles USING btree (pair_id, "interval", open_time);
CREATE UNIQUE INDEX index_ema1d_daily_checks_on_signal_date ON public.ema1d_daily_checks USING btree (signal_date);
CREATE UNIQUE INDEX index_ema1d_equity_snapshots_on_date ON public.ema1d_equity_snapshots USING btree (date);
CREATE UNIQUE INDEX index_ema1d_pairs_on_pair_id ON public.ema1d_pairs USING btree (pair_id);
CREATE INDEX index_ema1d_pairs_on_status ON public.ema1d_pairs USING btree (status);
CREATE UNIQUE INDEX index_ema1d_syncs_on_run_id_and_signal_date ON public.ema1d_syncs USING btree (run_id, signal_date) WHERE (dry_run = false);
CREATE INDEX index_ema1d_syncs_on_signal_date ON public.ema1d_syncs USING btree (signal_date);
CREATE INDEX index_ema1d_todos_on_depends_on_id ON public.ema1d_todos USING btree (depends_on_id);
CREATE INDEX index_ema1d_todos_on_ema1d_sync_id ON public.ema1d_todos USING btree (ema1d_sync_id);
CREATE INDEX index_ema1d_todos_on_ema_trade_id ON public.ema1d_todos USING btree (ema_trade_id);
CREATE INDEX index_ema1d_todos_on_status_and_run_after ON public.ema1d_todos USING btree (status, run_after);
CREATE UNIQUE INDEX index_ema1d_todos_uniqueness ON public.ema1d_todos USING btree (ema1d_pair_id, action, side, signal_date) WHERE ((status)::text <> 'CANCELLED'::text);
CREATE INDEX index_ema_settings_on_pair_id ON public.ema_settings USING btree (pair_id);
CREATE INDEX index_ema_settings_on_pair_id_and_status ON public.ema_settings USING btree (pair_id, status);
CREATE INDEX index_ema_settings_on_status ON public.ema_settings USING btree (status);
CREATE INDEX index_ema_trades_on_ema_setting_id ON public.ema_trades USING btree (ema_setting_id);
CREATE INDEX index_ema_trades_on_ema_setting_id_and_status ON public.ema_trades USING btree (ema_setting_id, status);
CREATE INDEX index_ema_trades_on_source_and_status ON public.ema_trades USING btree (source, status);
CREATE INDEX index_pairs_on_is_auto_trade ON public.pairs USING btree (is_auto_trade);
CREATE INDEX pairs_on_deleted_at ON public.pairs USING btree (deleted_at);
CREATE UNIQUE INDEX pairs_on_name ON public.pairs USING btree (name);
CREATE INDEX pairs_on_status ON public.pairs USING btree (status);
ALTER TABLE ONLY public.ema_trades
    ADD CONSTRAINT fk_rails_0e9f292780 FOREIGN KEY (pair_id) REFERENCES public.pairs(id);
ALTER TABLE ONLY public.pairs
    ADD CONSTRAINT fk_rails_39fdeaee16 FOREIGN KEY (coin_id) REFERENCES public.coins(id);
ALTER TABLE ONLY public.pairs
    ADD CONSTRAINT fk_rails_7e3c5777c8 FOREIGN KEY (market_id) REFERENCES public.markets(id);
ALTER TABLE ONLY public.dca_settings
    ADD CONSTRAINT fk_rails_dca_settings_pair FOREIGN KEY (pair_id) REFERENCES public.pairs(id);
ALTER TABLE ONLY public.dca_trades
    ADD CONSTRAINT fk_rails_dca_trades_setting FOREIGN KEY (dca_setting_id) REFERENCES public.dca_settings(id);
ALTER TABLE ONLY public.ema1d_candles
    ADD CONSTRAINT fk_rails_ema1d_candles_pair FOREIGN KEY (pair_id) REFERENCES public.pairs(id);
ALTER TABLE ONLY public.ema1d_pairs
    ADD CONSTRAINT fk_rails_ema1d_pairs_pair FOREIGN KEY (pair_id) REFERENCES public.pairs(id);
ALTER TABLE ONLY public.ema1d_todos
    ADD CONSTRAINT fk_rails_ema1d_todos_depends_on FOREIGN KEY (depends_on_id) REFERENCES public.ema1d_todos(id);
ALTER TABLE ONLY public.ema1d_todos
    ADD CONSTRAINT fk_rails_ema1d_todos_ema1d_pair FOREIGN KEY (ema1d_pair_id) REFERENCES public.ema1d_pairs(id);
ALTER TABLE ONLY public.ema1d_todos
    ADD CONSTRAINT fk_rails_ema1d_todos_ema1d_sync FOREIGN KEY (ema1d_sync_id) REFERENCES public.ema1d_syncs(id);
ALTER TABLE ONLY public.ema1d_todos
    ADD CONSTRAINT fk_rails_ema1d_todos_ema_trade FOREIGN KEY (ema_trade_id) REFERENCES public.ema_trades(id);
ALTER TABLE ONLY public.ema_settings
    ADD CONSTRAINT fk_rails_ema_settings_pair FOREIGN KEY (pair_id) REFERENCES public.pairs(id);
ALTER TABLE ONLY public.ema_trades
    ADD CONSTRAINT fk_rails_ema_trades_ema_setting FOREIGN KEY (ema_setting_id) REFERENCES public.ema_settings(id);
