// Run via: make dashboard-seed
// This seeds the NeoDash dashboard JSON from dashboards/wallet-graph.json
// into Neo4j as a :_Neodash_Dashboard node so NeoDash standalone mode
// can auto-load it.
//
// The Makefile target handles JSON escaping and passes content via neo4j's
// apoc.text or direct cypher parameter.
MERGE (d:_Neodash_Dashboard {uuid: "wallet-graph-v1"})
SET d.title = "Wallet Graph",
    d.version = "2.4",
    d.user = "neo4j",
    d.content = $content,
    d.date = datetime()
