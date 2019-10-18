## Scout ##

Scout is a daemon used to automate couchbase deployments and monitoring

How to get started
1. Clone this repo
2. Install consul and get the address of the consul instance
3. run ```go build . ``` to build a binary
4. edit the ```config.yml``` file in the root directory with your credentials.
5. copy `config.yml` to ```/etc/config.yml```
6. Run `./scout` and sit back, relax and grab a hot cup of coffee
`