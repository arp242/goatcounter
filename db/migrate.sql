-- 20190611; split out ref path and parameters
alter table hits
    add column ref_params;
update hits set
    ref_params = case instr(ref, "?")
                 when 0 then null
                 else substr(ref, instr(ref, "?") + 1)
                 end,
    ref        = case instr(ref, "?")
                 when 0 then ref
                 else substr(ref, 0, instr(ref, "?"))
                 end;


-- 20190611; leading / for paths sent from the client wasn't normalized.
update hits set path = "/" || ltrim(path, "/");

-- 20190611; remove hits from local dev.
delete from hits where ref like "%localhost%";

-- 20190615: remove trailing slash from refs, paths
update hits set
    path = case path
           when "/" then "/"
           else rtrim(path, "/")
           end,
    ref =  rtrim(ref, "/");
