import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant3 ===")
  val ns = `shared-bootstrap`.participantInit(participant, "participant3")
  logger.info("=== participant3 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant3") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }

  logger.info("=== uploading DARs to participant3 ===")
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
  logger.info("=== DARs uploaded to participant3 ===")

  logger.info("=== creating parties on participant3 ===")
  val charlieParty = participant.parties.enable("charlie", ns)
  logger.info(s"=== party created: charlie=${charlieParty} ===")

  participant.health.ping(participant)
  logger.info("=== finishing participant3 bootstrap ===")
}
