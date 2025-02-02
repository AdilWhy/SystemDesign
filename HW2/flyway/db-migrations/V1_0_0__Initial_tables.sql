create table if not exists public.token(
    client_id VARCHAR(50),
    access_scope varchar,
    access_token varchar default SUBSTR(UPPER(md5(random()::text)), 2, 22),
    expiration_time timestamptz default current_timestamp + interval '2hours',
    unique(client_id, access_scope)
);



create table if not exists public.user (
    client_id varchar(50) unique,
    client_secret varchar(100),
    scope varchar[]
);

