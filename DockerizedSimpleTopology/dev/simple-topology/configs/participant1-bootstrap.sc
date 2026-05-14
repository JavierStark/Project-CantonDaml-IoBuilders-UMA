import com.daml.nonempty.NonEmpty
import com.digitalasset.canton.console.LocalInstanceReference

def participantInit(node: LocalInstanceReference, label: String): Unit = {
  val namespaceKey =
    node.keys.secret
      .generate_signing_key(
        name = label + s"-${SigningKeyUsage.Namespace.identifier}",
        SigningKeyUsage.NamespaceOnly,
      )

  val sequencerAuthKey =
    node.keys.secret.generate_signing_key(
      name = label + s"-${SigningKeyUsage.SequencerAuthentication.identifier}",
      SigningKeyUsage.SequencerAuthenticationOnly,
    )

  val signingKey =
    node.keys.secret
      .generate_signing_key(
        name = label + s"-${SigningKeyUsage.Protocol.identifier}",
        SigningKeyUsage.ProtocolOnly,
      )

  val encryptionKey =
    node.keys.secret.generate_encryption_key(name = label + "-encryption")
  val namespace = Namespace(namespaceKey.id)
  node.topology.init_id_from_uid(
    UniqueIdentifier.tryCreate(label, namespace)
  )

  node.health.wait_for_ready_for_node_topology()

  node.topology.namespace_delegations.propose_delegation(
    namespace,
    namespaceKey,
    CanSignAllMappings,
  )

  node.topology.owner_to_key_mappings.propose(
    member = node.id.member,
    keys = NonEmpty(Seq, sequencerAuthKey, signingKey, encryptionKey),
    signedBy = Seq(namespaceKey.fingerprint, sequencerAuthKey.fingerprint, signingKey.fingerprint),
  )
  node.health.wait_for_initialized()
}

def main() = {
  logger.info("=== init participant1 ===")
  participantInit(participant, "participant1")
  logger.info("=== participant1 initialized ===")
  logger.info("=== connecting to synchronizer ===")
  var retries = 36
  var connected = false
  while (retries > 0 && !connected) {
    try {
      participant.synchronizers.connect("da", s"http://sequencer1:5001")
      connected = true
      logger.info("=== connected to synchronizer ===")
    } catch {
      case e: Throwable =>
        logger.info(s"Retrying synchronizer connect (${retries} left): ${e.getMessage}")
        Thread.sleep(5000)
        retries -= 1
    }
  }
  if (!connected) {
    throw new RuntimeException("Failed to connect to synchronizer after 36 retries")
  }
  participant.health.ping(participant)
  logger.info("=== finishing participant1 bootstrap ===")
}
