import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant2 ===")
  val ns = `shared-bootstrap`.participantInit(participant, "participant2")
  logger.info("=== participant2 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant2") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }

  logger.info("=== uploading DARs to participant2 ===")
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
  logger.info("=== DARs uploaded to participant2 ===")

  logger.info("=== creating parties on participant2 ===")
  val bobParty = participant.parties.enable("bob", ns)
  logger.info(s"=== party created: bob=${bobParty} ===")

  participant.health.ping(participant)
  logger.info("=== finishing participant2 bootstrap ===")
}
