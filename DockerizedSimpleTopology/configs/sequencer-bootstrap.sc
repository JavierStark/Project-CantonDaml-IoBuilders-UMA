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

def main() = nodeInit(sequencer)
