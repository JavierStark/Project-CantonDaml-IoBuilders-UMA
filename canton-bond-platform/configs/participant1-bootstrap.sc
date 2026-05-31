import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant1 ===")
  val ns = `shared-bootstrap`.participantInit(participant, "participant1")
  logger.info("=== participant1 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant1") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }

  logger.info("=== uploading DARs to participant1 ===")
  val spliceDars = Seq(
    "/app/dars/splice-api-token-metadata-v1-1.0.0.dar",
    "/app/dars/splice-api-token-holding-v1-1.0.0.dar",
    "/app/dars/splice-api-token-transfer-instruction-v1-1.0.0.dar",
    "/app/dars/splice-api-token-allocation-v1-1.0.0.dar",
    "/app/dars/splice-api-token-allocation-instruction-v1-1.0.0.dar",
    "/app/dars/simple-token-0.1.0.dar",
  )
  spliceDars.foreach { darPath =>
    logger.info(s"=== uploading $darPath ===")
    participant.dars.upload(darPath)
  }
  logger.info("=== DARs uploaded to participant1 ===")

  logger.info("=== creating parties on participant1 ===")
  val adminParty = participant.parties.enable("admin", ns)
  val aliceParty = participant.parties.enable("alice", ns)
  val executorParty = participant.parties.enable("executor", ns)
  logger.info(s"=== parties created: admin=${adminParty}, alice=${aliceParty}, executor=${executorParty} ===")

  participant.health.ping(participant)
  logger.info("=== finishing participant1 bootstrap ===")
}
