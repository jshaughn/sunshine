# Vizceral Investigation

This is just an example of how we could potentially use Vizceral in SWS.  The code here
queries Prometheus when running Istio and uses the istio_request_count metric to
generate a svc graph for the active services.

To try this out:

* Run the Istio bookinfo demo app
  * Generate traffic (Set PRODUCTPAGE to the exposed productpage svc, like maybe productpage-istio-system.127.0.0.1.nip.io)
  * watch -n 1 curl -o /dev/null -s -w %{http_code}\n $PRODUCTPAGE/productpage

* Setup the Vizceral example app (I cloned it)
  * https://github.com/Netflix/vizceral-example

```
git clone git@github.com:Netflix/vizceral-example.git
cd vizceral-example
npm install
```

* Install `sunshine` into your go env (use a temp GOPATH if you want to keep this isolated)
  * `go get github.com/jshaughn/sunshine`

* Run `sunshine` and specify the promHost running in your istio env
  * `$GOPATH/bin/sunshine -server http://<promHost>:9090`

* This will dump out a JSON config, cut and paste it, replacing the contents of
  * `vizceral-example/sample_data.json`

* Start Vizceral:
  * `npm run dev`

* Take a look at `localhost:8080`
  * Drill down into the mess by clicking
  * Isolate paths in the service graph by hovering over a node
  * See details by single-click a connection or node
  * center view on a node by dbl-click
  * click any node and in the detail popup there should be a link to the prometheus graph for it's requests
    * this is just an example of linking out from the graph

* Notes
  * This is not currently refreshing the service graph, it is just showing the view at the time of the run
  * The code could certainly be cleaned up





