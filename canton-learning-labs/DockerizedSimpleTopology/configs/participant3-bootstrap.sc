import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant3 ===")
  `shared-bootstrap`.participantInit(participant, "participant3")
  logger.info("=== participant3 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant3") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }
  participant.health.ping(participant)
  logger.info("=== finishing participant3 bootstrap ===")
}
