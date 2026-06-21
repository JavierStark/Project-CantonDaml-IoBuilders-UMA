import $file.`shared-bootstrap`

def main() = {
  logger.info("=== init participant2 ===")
  `shared-bootstrap`.participantInit(participant, "participant2")
  logger.info("=== participant2 initialized ===")
  `shared-bootstrap`.connectToSynchronizer("participant2") {
    participant.synchronizers.connect("da", "http://sequencer1:5001")
  }
  participant.health.ping(participant)
  logger.info("=== finishing participant2 bootstrap ===")
}
