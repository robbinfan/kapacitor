# File based task/template/handler definition

This proposal introduces directory based task, template, and handler definition.

## Configuration

```
[load]
 enabled = true
 hard = false
 dir="/path/to/directory"
```

`dir` specifies the directory where the definition files exist.

The specified directory should have three subdirectories `tasks`, `templates`, and `handlers`.

The `tasks` directory will contain will contain task tickscripts.

The `templates` directory will contain both template tickscripts and the associated templated task
definition files (either yaml or json).

The `handlers` directory will contain will contain all topic handler definitions in yaml or json.

## tickscript (files with a `.tick` extension)

Tickscripts can be marked up with commented key-value pairs for predefined attributes.
Those attributes may be any of the following

* `id` - the file name without the tick extension
* `type` - determined by introspection of the task (stream, batch)
* `kind` - task, template. defined using a `set statement`
* `subscriptions` - defined using the `subscribe` keyword followed by a specified subscription

For example
```
dbrp "telegraf"."autogen"

var measurement string
var where_filter = lambda: TRUE
var groups = [*]
var field string
var warn lambda
var crit lambda
var window = 5m
var slack_channel = '#alerts'

stream
    |from()
        .measurement(measurement)
        .where(where_filter)
        .groupBy(groups)
    |window()
        .period(window)
        .every(window)
    |mean(field)
    |alert()
         .warn(warn)
         .crit(crit)
         .slack()
         .channel(slack_channel)
```

or

```
dbrp "telegraf"."autogen"

stream
    |from()
        .measurement('cpu')
        .groupBy(*)
    |alert()
        .warn(lambda: "usage_idle" < 20)
        .crit(lambda: "usage_idle" < 10)
        // Send alerts to the `cpu` topic
        .topic('cpu')
```

### Template Vars

Template variables may be added as either json or yaml.

* `id` - filename without the `yaml` or `yml` extension
* `dbrp` - required
* `template` - required
* `vars` - list of template vars

```yaml
dbrp:
  - telegraf.autogen
  - telegraf.not_autogen
template: base_template
vars: {
  "measurement": {"type" : "string", "value" : "cpu" },
  "where_filter": {"type": "lambda", "value": "\"cpu\" == 'cpu-total'"},
  "groups": {"type": "list", "value": [{"type":"string", "value":"host"},{"type":"string", "value":"dc"}]},
  "field": {"type" : "string", "value" : "usage_idle" },
  "warn": {"type" : "lambda", "value" : "\"mean\" < 30.0" },
  "crit": {"type" : "lambda", "value" : "\"mean\" < 10.0" },
  "window": {"type" : "duration", "value" : "1m" },
  "slack_channel": {"type" : "string", "value" : "#alerts_testing" }
}
```

or

```json
{
  "dbrp": ["telegraf.autogen"],
  "template": "base_template",
  "vars": {
    "measurement": {"type" : "string", "value" : "cpu" },
    "where_filter": {"type": "lambda", "value": "\"cpu\" == 'cpu-total'"},
    "groups": {"type": "list", "value": [{"type":"string", "value":"host"},{"type":"string", "value":"dc"}]},
    "field": {"type" : "string", "value" : "usage_idle" },
    "warn": {"type" : "lambda", "value" : "\"mean\" < 30.0" },
    "crit": {"type" : "lambda", "value" : "\"mean\" < 10.0" },
    "window": {"type" : "duration", "value" : "1m" },
    "slack_channel": {"type" : "string", "value" : "#alerts_testing" }
  }
}
```

## Handlers

Topic handlers must specify its associtated topic like so

```
topic: cpu
kind: slack
match: changed() == TRUE
options:
  channel: '#alerts'
```

the name of the file will be used as the handler id.

