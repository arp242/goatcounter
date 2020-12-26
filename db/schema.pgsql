create table sites (
	site_id        serial         primary key,
	parent         integer        null,

	code           varchar        not null                 check(length(code) >= 2 and length(code) <= 50),
	link_domain    varchar        not null default ''      check(link_domain = '' or (length(link_domain) >= 4 and length(link_domain) <= 255)),
	cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
	cname_setup_at timestamp      default null             ,
	plan           varchar        not null                 check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom')),
	stripe         varchar        null,
	billing_amount varchar,
	settings       jsonb          not null,
	received_data  integer        not null default 0,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null                 ,
	updated_at     timestamp                               ,
	first_hit_at   timestamp      not null                 
);
create unique index "sites#code"   on sites(lower(code));
create unique index "sites#cname"  on sites(lower(cname));
create        index "sites#parent" on sites(parent) where state='a';

create table users (
	user_id        serial         primary key,
	site_id        integer        not null,

	email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
	email_verified integer        not null default 0,
	password       bytea          default null,
	totp_enabled   integer        not null default 0,
	totp_secret    bytea   ,
	role           varchar        not null default ''      check(role in ('', 'a')),
	login_at       timestamp      null                     ,
	login_request  varchar        null,
	login_token    varchar        null,
	csrf_token     varchar        null,
	email_token    varchar        null,
	seen_updates_at timestamp     not null default current_timestamp ,
	reset_at       timestamp      null,

	created_at     timestamp      not null                 ,
	updated_at     timestamp                               ,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
create        index "users#site_id"       on users(site_id);
create unique index "users#site_id#email" on users(site_id, lower(email));

create table api_tokens (
	api_token_id   serial         primary key,
	site_id        integer        not null,
	user_id        integer        not null,

	name           varchar        not null,
	token          varchar        not null                 check(length(token) > 10),
	permissions    jsonb          not null,
	created_at     timestamp      not null                 ,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	foreign key (user_id) references users(user_id) on delete restrict on update restrict
);
create unique index "api_tokens#site_id#token" on api_tokens(site_id, token);

create table hits (
	hit_id         serial         primary key,
	-- No foreign keys on this as checking them for every insert is
	-- comparatively expensive.
	site_id        integer        not null                 check(site_id > 0),
	path_id        integer        not null                 check(path_id > 0),
	user_agent_id  integer        null                     check(user_agent_id != 0),

	session        bytea          default null,
	bot            integer        default 0,
	ref            varchar        not null,
	ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
	size           varchar        not null default '',
	location       varchar        not null default '',
	first_visit    integer        default 0,

	created_at     timestamp      not null                 
);
create index "hits#site_id#created_at" on hits(site_id, created_at desc);
cluster hits using "hits#site_id#created_at";

create table paths (
	path_id        serial         primary key,
	site_id        integer        not null,

	path           varchar        not null,
	title          varchar        not null default '',
	event          integer        default 0,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
create unique index "paths#site_id#path" on paths(site_id, lower(path));
create        index "paths#path#title"   on paths(lower(path), lower(title));

create table browsers (
	browser_id     serial         primary key,

	name           varchar,
	version        varchar
);

create table systems (
	system_id      serial         primary key,

	name           varchar,
	version        varchar
);

create table user_agents (
	user_agent_id  serial         primary key,
	browser_id     integer        not null,
	system_id      integer        not null,

	ua             varchar        not null,
	isbot          integer        not null,

	foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict,
	foreign key (system_id)  references systems(system_id)   on delete restrict on update restrict
);
create unique index "user_agents#ua" on user_agents(ua);

create table hit_counts (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.

	hour           timestamp      not null                 ,
	total          integer        not null,
	total_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour) 
);
create index "hit_counts#site_id#hour" on hit_counts(site_id, hour desc);
cluster hit_counts using "hit_counts#site_id#hour";
alter table hit_counts replica identity using index "hit_counts#site_id#path_id#hour";

create table ref_counts (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.

	ref            varchar        not null,
	ref_scheme     varchar        null,
	hour           timestamp      not null                 ,
	total          integer        not null,
	total_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour) 
);
create index "ref_counts#site_id#hour" on ref_counts(site_id, hour);
cluster ref_counts using "ref_counts#site_id#hour";
alter table ref_counts replica identity using index "ref_counts#site_id#path_id#ref#hour";

create table hit_stats (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.

	day            date           not null                 ,
	stats          varchar        not null,
	stats_unique   varchar        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "hit_stats#site_id#path_id#day" unique(site_id, path_id, day) 
);
create index "hit_stats#site_id#day" on hit_stats(site_id, day desc);
cluster hit_stats using "hit_stats#site_id#day";
alter table hit_stats replica identity using index "hit_stats#site_id#path_id#day";

create table browser_stats (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.
	browser_id     integer        not null,

	day            date           not null                 ,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id)    references sites(site_id)       on delete restrict on update restrict,
	foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict,
	constraint "browser_stats#site_id#path_id#day#browser_id" unique(site_id, path_id, day, browser_id) 
);
create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day desc);
cluster browser_stats using "browser_stats#site_id#path_id#day#browser_id";
alter table browser_stats replica identity using index "browser_stats#site_id#path_id#day#browser_id";

create table system_stats (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.
	system_id      integer        not null,

	day            date           not null                 ,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id)   references sites(site_id)     on delete restrict on update restrict,
	foreign key (system_id) references systems(system_id) on delete restrict on update restrict,
	constraint "system_stats#site_id#path_id#day#system_id" unique(site_id, path_id, day, system_id) 
);
create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day desc);
cluster system_stats using "system_stats#site_id#path_id#day#system_id";
alter table system_stats replica identity using index "system_stats#site_id#path_id#day#system_id";

create table location_stats (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.

	day            date           not null                 ,
	location       varchar        not null,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "location_stats#site_id#path_id#day#location" unique(site_id, path_id, day, location) 
);
create index "location_stats#site_id#day" on location_stats(site_id, day desc);
cluster location_stats using "location_stats#site_id#day";
alter table location_stats replica identity using index "location_stats#site_id#path_id#day#location";

create table size_stats (
	site_id        integer        not null,
	path_id        integer        not null,  -- No FK for performance.

	day            date           not null                 ,
	width          integer        not null,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "size_stats#site_id#path_id#day#width" unique(site_id, path_id, day, width) 
);
create index "size_stats#site_id#day" on size_stats(site_id, day desc);
cluster size_stats using "size_stats#site_id#day";
alter table size_stats replica identity using index "size_stats#site_id#path_id#day#width";

create table updates (
	id             serial         primary key,
	subject        varchar        not null,
	body           varchar        not null,

	created_at     timestamp      not null                 ,
	show_at        timestamp      not null                 
);
create index "updates#show_at" on updates(show_at);

create table exports (
	export_id      serial         primary key,
	site_id        integer        not null,
	start_from_hit_id integer     not null,

	path           varchar        not null,
	created_at     timestamp      not null                 ,

	finished_at    timestamp                               ,
	last_hit_id    integer,
	num_rows       integer,
	size           varchar,
	hash           varchar,
	error          varchar,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
create index "exports#site_id#created_at" on exports(site_id, created_at);

create table store (
	key            varchar        not null,
	value          text
);
create unique index "store#key" on store(key);
alter table store replica identity using index "store#key";

create table iso_3166_1 (
	name           varchar,
	alpha2          varchar
);
create unique index "iso_3166_1#alpha2" on iso_3166_1(alpha2);

insert into iso_3166_1 (name, alpha2) values
	('(unknown)', ''),

	('Ascension Island', 'AC'),
	('Andorra', 'AD'),
	('United Arab Emirates', 'AE'),
	('Afghanistan', 'AF'),
	('Antigua and Barbuda', 'AG'),
	('Anguilla', 'AI'),
	('Albania', 'AL'),
	('Armenia', 'AM'),
	('Netherlands Antilles', 'AN'),
	('Angola', 'AO'),
	('Antarctica', 'AQ'),
	('Argentina', 'AR'),
	('American Samoa', 'AS'),
	('Austria', 'AT'),
	('Australia', 'AU'),
	('Aruba', 'AW'),
	('Åland Islands', 'AX'),
	('Azerbaijan', 'AZ'),
	('Bosnia and Herzegovina', 'BA'),
	('Barbados', 'BB'),
	('Bangladesh', 'BD'),
	('Belgium', 'BE'),
	('Burkina Faso', 'BF'),
	('Bulgaria', 'BG'),
	('Bahrain', 'BH'),
	('Burundi', 'BI'),
	('Benin', 'BJ'),
	('Saint Barthélemy', 'BL'),
	('Bermuda', 'BM'),
	('Brunei Darussalam', 'BN'),
	('Bolivia', 'BO'),
	('Bonaire, Sint Eustatius and Saba', 'BQ'),
	('Brazil', 'BR'),
	('Bahamas', 'BS'),
	('Bhutan', 'BT'),
	('Burma', 'BU'),
	('Bouvet Island', 'BV'),
	('Botswana', 'BW'),
	('Belarus', 'BY'),
	('Belize', 'BZ'),
	('Canada', 'CA'),
	('Cocos (Keeling) Islands', 'CC'),
	('Democratic Republic of thje Congo', 'CD'),
	('Central African Republic', 'CF'),
	('Congo', 'CG'),
	('Switzerland', 'CH'),
	('Côte d''Ivoire', 'CI'),
	('Cook Islands', 'CK'),
	('Chile', 'CL'),
	('Cameroon', 'CM'),
	('China', 'CN'),
	('Colombia', 'CO'),
	('Clipperton Island', 'CP'),
	('Costa Rica', 'CR'),
	('Serbia and Montenegro', 'CS'),
	('Cuba', 'CU'),
	('Cabo Verde', 'CV'),
	('Curaçao', 'CW'),
	('Christmas Island', 'CX'),
	('Cyprus', 'CY'),
	('Czechia', 'CZ'),
	('Germany', 'DE'),
	('Diego Garcia', 'DG'),
	('Djibouti', 'DJ'),
	('Denmark', 'DK'),
	('Dominica', 'DM'),
	('Dominican Republic', 'DO'),
	('Benin', 'DY'),
	('Algeria', 'DZ'),
	('Ceuta, Melilla', 'EA'),
	('Ecuador', 'EC'),
	('Estonia', 'EE'),
	('Egypt', 'EG'),
	('Western Sahara', 'EH'),
	('Eritrea', 'ER'),
	('Spain', 'ES'),
	('Ethiopia', 'ET'),
	('European Union', 'EU'),
	('Estonia', 'EW'),
	('Eurozone', 'EZ'),
	('Finland', 'FI'),
	('Fiji', 'FJ'),
	('Falkland Islands (Malvinas)', 'FK'),
	('Liechtenstein', 'FL'),
	('Micronesia', 'FM'),
	('Faroe Islands', 'FO'),
	('France', 'FR'),
	('France, Metropolitan', 'FX'),
	('Gabon', 'GA'),
	('United Kingdom', 'GB'),
	('Grenada', 'GD'),
	('Georgia', 'GE'),
	('French Guiana', 'GF'),
	('Guernsey', 'GG'),
	('Ghana', 'GH'),
	('Gibraltar', 'GI'),
	('Greenland', 'GL'),
	('Gambia', 'GM'),
	('Guinea', 'GN'),
	('Guadeloupe', 'GP'),
	('Equatorial Guinea', 'GQ'),
	('Greece', 'GR'),
	('South Georgia and the South Sandwich Islands', 'GS'),
	('Guatemala', 'GT'),
	('Guam', 'GU'),
	('Guinea-Bissau', 'GW'),
	('Guyana', 'GY'),
	('Hong Kong', 'HK'),
	('Heard Island and McDonald Islands', 'HM'),
	('Honduras', 'HN'),
	('Croatia', 'HR'),
	('Haiti', 'HT'),
	('Hungary', 'HU'),
	('Canary Islands', 'IC'),
	('Indonesia', 'ID'),
	('Ireland', 'IE'),
	('Israel', 'IL'),
	('Isle of Man', 'IM'),
	('India', 'IN'),
	('British Indian Ocean Territory', 'IO'),
	('Iraq', 'IQ'),
	('Iran', 'IR'),
	('Iceland', 'IS'),
	('Italy', 'IT'),
	('Jamaica', 'JA'),
	('Jersey', 'JE'),
	('Jamaica', 'JM'),
	('Jordan', 'JO'),
	('Japan', 'JP'),
	('Kenya', 'KE'),
	('Kyrgyzstan', 'KG'),
	('Cambodia', 'KH'),
	('Kiribati', 'KI'),
	('Comoros', 'KM'),
	('Saint Kitts and Nevis', 'KN'),
	('North Korea', 'KP'),
	('South Korea', 'KR'),
	('Kuwait', 'KW'),
	('Cayman Islands', 'KY'),
	('Kazakhstan', 'KZ'),
	('Lao People''s Democratic Republic', 'LA'),
	('Lebanon', 'LB'),
	('Saint Lucia', 'LC'),
	('Libya Fezzan', 'LF'),
	('Liechtenstein', 'LI'),
	('Sri Lanka', 'LK'),
	('Liberia', 'LR'),
	('Lesotho', 'LS'),
	('Lithuania', 'LT'),
	('Luxembourg', 'LU'),
	('Latvia', 'LV'),
	('Libya', 'LY'),
	('Morocco', 'MA'),
	('Monaco', 'MC'),
	('Moldova, Republic of', 'MD'),
	('Montenegro', 'ME'),
	('Saint Martin (French part)', 'MF'),
	('Madagascar', 'MG'),
	('Marshall Islands', 'MH'),
	('North Macedonia', 'MK'),
	('Mali', 'ML'),
	('Myanmar', 'MM'),
	('Mongolia', 'MN'),
	('Macao', 'MO'),
	('Northern Mariana Islands', 'MP'),
	('Martinique', 'MQ'),
	('Mauritania', 'MR'),
	('Montserrat', 'MS'),
	('Malta', 'MT'),
	('Mauritius', 'MU'),
	('Maldives', 'MV'),
	('Malawi', 'MW'),
	('Mexico', 'MX'),
	('Malaysia', 'MY'),
	('Mozambique', 'MZ'),
	('Namibia', 'NA'),
	('New Caledonia', 'NC'),
	('Niger', 'NE'),
	('Norfolk Island', 'NF'),
	('Nigeria', 'NG'),
	('Nicaragua', 'NI'),
	('Netherlands', 'NL'),
	('Norway', 'NO'),
	('Nepal', 'NP'),
	('Nauru', 'NR'),
	('Neutral Zone', 'NT'),
	('Niue', 'NU'),
	('New Zealand', 'NZ'),
	('Oman', 'OM'),
	('Escape code', 'OO'),
	('Panama', 'PA'),
	('Peru', 'PE'),
	('French Polynesia', 'PF'),
	('Papua New Guinea', 'PG'),
	('Philippines', 'PH'),
	('Philippines', 'PI'),
	('Pakistan', 'PK'),
	('Poland', 'PL'),
	('Saint Pierre and Miquelon', 'PM'),
	('Pitcairn', 'PN'),
	('Puerto Rico', 'PR'),
	('Palestine, State of', 'PS'),
	('Portugal', 'PT'),
	('Palau', 'PW'),
	('Paraguay', 'PY'),
	('Qatar', 'QA'),
	('Argentina', 'RA'),
	('Bolivia; Botswana', 'RB'),
	('China', 'RC'),
	('Réunion', 'RE'),
	('Haiti', 'RH'),
	('Indonesia', 'RI'),
	('Lebanon', 'RL'),
	('Madagascar', 'RM'),
	('Niger', 'RN'),
	('Romania', 'RO'),
	('Philippines', 'RP'),
	('Serbia', 'RS'),
	('Russian Federation', 'RU'),
	('Rwanda', 'RW'),
	('Saudi Arabia', 'SA'),
	('Solomon Islands', 'SB'),
	('Seychelles', 'SC'),
	('Sudan', 'SD'),
	('Sweden', 'SE'),
	('Finland', 'SF'),
	('Singapore', 'SG'),
	('Saint Helena, Ascension and Tristan da Cunha', 'SH'),
	('Slovenia', 'SI'),
	('Svalbard and Jan Mayen', 'SJ'),
	('Slovakia', 'SK'),
	('Sierra Leone', 'SL'),
	('San Marino', 'SM'),
	('Senegal', 'SN'),
	('Somalia', 'SO'),
	('Suriname', 'SR'),
	('South Sudan', 'SS'),
	('Sao Tome and Principe', 'ST'),
	('USSR', 'SU'),
	('El Salvador', 'SV'),
	('Sint Maarten (Dutch part)', 'SX'),
	('Syrian Arab Republic', 'SY'),
	('Eswatini', 'SZ'),
	('Tristan da Cunha', 'TA'),
	('Turks and Caicos Islands', 'TC'),
	('Chad', 'TD'),
	('French Southern Territories', 'TF'),
	('Togo', 'TG'),
	('Thailand', 'TH'),
	('Tajikistan', 'TJ'),
	('Tokelau', 'TK'),
	('Timor-Leste', 'TL'),
	('Turkmenistan', 'TM'),
	('Tunisia', 'TN'),
	('Tonga', 'TO'),
	('East Timor', 'TP'),
	('Turkey', 'TR'),
	('Trinidad and Tobago', 'TT'),
	('Tuvalu', 'TV'),
	('Taiwan', 'TW'),
	('Tanzania', 'TZ'),
	('Ukraine', 'UA'),
	('Uganda', 'UG'),
	('United Kingdom', 'UK'),
	('United States Minor Outlying Islands', 'UM'),
	('United Nations', 'UN'),
	('United States', 'US'),
	('Uruguay', 'UY'),
	('Uzbekistan', 'UZ'),
	('Holy See', 'VA'),
	('Saint Vincent and the Grenadines', 'VC'),
	('Venezuela', 'VE'),
	('Virgin Islands (British)', 'VG'),
	('Virgin Islands (U.S.)', 'VI'),
	('Viet Nam', 'VN'),
	('Vanuatu', 'VU'),
	('Wallis and Futuna', 'WF'),
	('Grenada', 'WG'),
	('Saint Lucia', 'WL'),
	('Samoa', 'WS'),
	('Saint Vincent', 'WV'),
	('Yemen', 'YE'),
	('Mayotte', 'YT'),
	('Yugoslavia', 'YU'),
	('Venezuela', 'YV'),
	('South Africa', 'ZA'),
	('Zambia', 'ZM'),
	('Zaire', 'ZR'),
	('Zimbabwe', 'ZW');

create table version (name varchar);
insert into version values
	('2020-08-28-1-paths-tables'),
	('2020-08-28-2-paths-paths'),
	('2020-08-28-3-paths-rmold'),
	('2020-08-28-4-user_agents'),
	('2020-08-28-5-paths-ua-fk'),
	('2020-08-28-6-paths-views'),
	('2020-12-11-1-constraint'),
	('2020-12-15-1-widgets.sql'),
	('2020-12-17-1-paths-isbot'),
	('2020-12-21-1-view'),
	('2020-12-24-1-user_agent_id_null');


-- vim:ft=sql:tw=0
