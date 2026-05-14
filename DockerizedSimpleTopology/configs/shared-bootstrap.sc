import com.daml.nonempty.NonEmpty
import com.digitalasset.canton.console.LocalInstanceReference

def nodeInit(node: LocalInstanceReference): Unit = {
  val namespaceKey = node.keys.secret
    .generate_signing_key(
      s"${node.name}-${SigningKeyUsage.Namespace.identifier}",
      usage = SigningKeyUsage.NamespaceOnly,
    )

  node.health.wait_for_ready_for_id()

  node.topology.init_id_from_uid(
    UniqueIdentifier.tryCreate(node.name, namespaceKey.fingerprint)
  )

  node.health.wait_for_ready_for_node_topology()

  node.topology.namespace_delegations.propose_delegation(
    Namespace(namespaceKey.fingerprint),
    namespaceKey,
    CanSignAllMappings,
  )

  val protocolSigningKey = node.keys.secret
    .generate_signing_key(
      s"${node.name}-${SigningKeyUsage.Protocol.identifier}",
      usage = SigningKeyUsage.ProtocolOnly,
    )

  val sequencerAuthKey = node.keys.secret
    .generate_signing_key(
      s"${node.name}-${SigningKeyUsage.SequencerAuthentication.identifier}",
      usage = SigningKeyUsage.SequencerAuthenticationOnly,
    )

  val keys = NonEmpty(Seq, protocolSigningKey, sequencerAuthKey)

  node.topology.owner_to_key_mappings.propose(
    member = node.id.member,
    keys = keys,
    signedBy = (namespaceKey +: keys).map(_.fingerprint),
  )

  node.health.wait_for_ready_for_initialization()
}

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

def connectToSynchronizer(label: String)(connect: => Unit): Unit = {
  logger.info(s"=== connecting $label to synchronizer ===")
  var retries = 36
  var connected = false
  while (retries > 0 && !connected) {
    try {
      connect
      connected = true
      logger.info(s"=== $label connected to synchronizer ===")
    } catch {
      case e: Throwable =>
        logger.info(s"Retrying synchronizer connect for $label (${retries} left): ${e.getMessage}")
        Thread.sleep(5000)
        retries -= 1
    }
  }
  if (!connected) {
    throw new RuntimeException(s"$label failed to connect to synchronizer after 36 retries")
  }
}
