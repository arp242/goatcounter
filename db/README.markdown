Create the PostgreSQL schema rom the SQLite version with:

    :%s/integer        primary key autoincrement/serial         primary key,/
    :%s/\s\+check(.\+strftime.\{-}\(,\?\)$/\1
