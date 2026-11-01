-- Argus template state machine (docs/SPEC.md §5). KEYS[1] = template hash key.
-- ARGV: template, level, now_rfc3339, masked_sample, grace_deadline_rfc3339,
--       regression_threshold_sec, surge_multipliers_json, ttl_seconds, sample_max

local key = KEYS[1]
local template = ARGV[1]
local level = ARGV[2]
local now_str = ARGV[3]
local sample_line = ARGV[4]
local grace_deadline = ARGV[5]
local regression_sec = tonumber(ARGV[6])
local surge_json = ARGV[7]
local ttl_seconds = tonumber(ARGV[8])
local sample_max = tonumber(ARGV[9])

local cjson = cjson or require("cjson")

local function is_leap(y)
  return (y % 4 == 0 and y % 100 ~= 0) or (y % 400 == 0)
end

local function parse_rfc3339(s)
  if not s or s == "" then
    return nil
  end
  local y, mo, d, h, mi, se = s:match("^(%d%d%d%d)%-(%d%d)%-(%d%d)T(%d%d):(%d%d):(%d%d)")
  if not y then
    return nil
  end
  y, mo, d, h, mi, se = tonumber(y), tonumber(mo), tonumber(d), tonumber(h), tonumber(mi), tonumber(se)
  local month_days = { 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31 }
  local days = 0
  for yr = 1970, y - 1 do
    days = days + 365 + (is_leap(yr) and 1 or 0)
  end
  for m = 1, mo - 1 do
    days = days + month_days[m]
    if m == 2 and is_leap(y) then
      days = days + 1
    end
  end
  days = days + (d - 1)
  return days * 86400 + h * 3600 + mi * 60 + se
end

local function grace_active(now_ts)
  if grace_deadline == nil or grace_deadline == "" then
    return false
  end
  local deadline_ts = parse_rfc3339(grace_deadline)
  if deadline_ts == nil then
    return false
  end
  return now_ts < deadline_ts
end

local function benign_level(lvl)
  return lvl == "DEBUG" or lvl == "INFO"
end

local function append_sample(existing_json, line, max_n)
  local samples = {}
  if existing_json and existing_json ~= "" then
    local ok, parsed = pcall(cjson.decode, existing_json)
    if ok and type(parsed) == "table" then
      samples = parsed
    end
  end
  table.insert(samples, line)
  while #samples > max_n do
    table.remove(samples, 1)
  end
  return cjson.encode(samples)
end

local function samples_array(existing_json, line, max_n)
  local encoded = append_sample(existing_json, line, max_n)
  local arr = cjson.decode(encoded)
  return encoded, arr
end

local now_ts = parse_rfc3339(now_str)
if now_ts == nil then
  return cjson.encode({ error = "invalid now_rfc3339" })
end

local surge_multipliers = cjson.decode(surge_json)
if type(surge_multipliers) ~= "table" then
  surge_multipliers = { 10, 100, 1000 }
end

local exists = redis.call("EXISTS", key)

local fire_webhook = false
local anomaly_type = ""
local surge_multiplier = 0
local out_reported_count = 0

if exists == 0 then
  local g = grace_active(now_ts)
  local reported_count = "0"
  local last_surge_tier = "0"
  local count = "1"

  if g and benign_level(level) then
    fire_webhook = false
  elseif g and not benign_level(level) then
    fire_webhook = true
    anomaly_type = "NEW_SIGNATURE"
    reported_count = "1"
    out_reported_count = 0
  else
    fire_webhook = true
    anomaly_type = "NEW_SIGNATURE"
    reported_count = "1"
    out_reported_count = 0
  end

  local samples_json, samples_arr = samples_array("", sample_line, sample_max)

  redis.call(
    "HSET",
    key,
    "status",
    "known",
    "first_seen",
    now_str,
    "last_seen",
    now_str,
    "count",
    count,
    "reported_count",
    reported_count,
    "last_surge_tier",
    last_surge_tier,
    "template",
    template,
    "level",
    level,
    "samples",
    samples_json
  )
  redis.call("EXPIRE", key, ttl_seconds)

  return cjson.encode({
    fire_webhook = fire_webhook,
    anomaly_type = anomaly_type,
    count = tonumber(count),
    reported_count = out_reported_count,
    surge_multiplier = surge_multiplier,
    sample_lines = samples_arr,
    first_seen = now_str,
    last_seen = now_str,
  })
end

local status = redis.call("HGET", key, "status") or "known"
local prev_last_seen = redis.call("HGET", key, "last_seen") or now_str
local first_seen = redis.call("HGET", key, "first_seen") or now_str
local count_str = redis.call("HGET", key, "count") or "0"
local reported_str = redis.call("HGET", key, "reported_count") or "0"
local last_surge_str = redis.call("HGET", key, "last_surge_tier") or "0"
local existing_samples = redis.call("HGET", key, "samples") or ""

local count = tonumber(count_str) + 1
local reported_count = tonumber(reported_str)
local last_surge_tier = tonumber(last_surge_str)

if status == "ignored" then
  local samples_json, samples_arr = samples_array(existing_samples, sample_line, sample_max)
  redis.call(
    "HSET",
    key,
    "count",
    tostring(count),
    "last_seen",
    now_str,
    "level",
    level,
    "samples",
    samples_json
  )
  redis.call("EXPIRE", key, ttl_seconds)
  return cjson.encode({
    fire_webhook = false,
    anomaly_type = "",
    count = count,
    reported_count = reported_count,
    surge_multiplier = 0,
    sample_lines = samples_arr,
    first_seen = first_seen,
    last_seen = now_str,
  })
end

local g = grace_active(now_ts)
local samples_json, samples_arr = samples_array(existing_samples, sample_line, sample_max)

if g and benign_level(level) then
  redis.call(
    "HSET",
    key,
    "count",
    tostring(count),
    "last_seen",
    now_str,
    "level",
    level,
    "samples",
    samples_json
  )
  redis.call("EXPIRE", key, ttl_seconds)
  return cjson.encode({
    fire_webhook = false,
    anomaly_type = "",
    count = count,
    reported_count = reported_count,
    surge_multiplier = 0,
    sample_lines = samples_arr,
    first_seen = first_seen,
    last_seen = now_str,
  })
end

local prev_ts = parse_rfc3339(prev_last_seen)
local regression_fired = false
if prev_ts ~= nil and regression_sec ~= nil and regression_sec > 0 then
  if (now_ts - prev_ts) >= regression_sec then
    fire_webhook = true
    anomaly_type = "REGRESSION"
    regression_fired = true
    out_reported_count = reported_count
    last_surge_tier = 0
    reported_count = count
  end
end

if not regression_fired then
  for _, m in ipairs(surge_multipliers) do
    local tier = tonumber(m)
    if tier and tier > last_surge_tier and reported_count >= 1 then
      if count >= reported_count * tier then
        fire_webhook = true
        anomaly_type = "FREQUENCY_SPIKE"
        surge_multiplier = tier
        out_reported_count = reported_count
        last_surge_tier = tier
        reported_count = count
        break
      end
    end
  end
end

redis.call(
  "HSET",
  key,
  "count",
  tostring(count),
  "last_seen",
  now_str,
  "level",
  level,
  "samples",
  samples_json,
  "reported_count",
  tostring(reported_count),
  "last_surge_tier",
  tostring(last_surge_tier)
)
if template ~= "" then
  redis.call("HSET", key, "template", template)
end
redis.call("EXPIRE", key, ttl_seconds)

return cjson.encode({
  fire_webhook = fire_webhook,
  anomaly_type = anomaly_type,
  count = count,
  reported_count = out_reported_count,
  surge_multiplier = surge_multiplier,
  sample_lines = samples_arr,
  first_seen = first_seen,
  last_seen = now_str,
})
