import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant1 ===")
  `shared-bootstrap`.participantInit(participant, "participant1")
  logger.info("=== participant1 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant1") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }
  participant.health.ping(participant)
  logger.info("=== finishing participant1 bootstrap ===")
}
