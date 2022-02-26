# Playground Instructions

This directory contains some sample config yml files and sample data to run some tests.

The flowlogs-pipeline application requires a config yml file and requires an input source.
The input source is specified in the yml file.
A simple test program is to run the following (from the base directory).

```
flowlogs-pipeline playground/aws1.yml
```

The aws? examples use a small file of prepared aws flow logs.
This is specified in the config file by specifying `pipeline.ingest.type: file`, `pipeline.decode.type: aws` and
specifying an ordered list of field names under `pipeline.decode.aws.fields`.

The aws1 example uses an explicit layout of the version 2 flow log format.
The aws2 example uses the default aws version 2 flow log format.
The aws3 example uses an explicit custom layout of the version 3 flow log format.
