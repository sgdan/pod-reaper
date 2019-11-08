package org.sgdan.podreaper

import io.micronaut.context.annotation.Requires
import java.lang.System.currentTimeMillis
import javax.inject.Singleton
import kotlin.random.Random

val adjectives = listOf("creepy", "bold", "angry", "beautiful", "serene")
val nouns = listOf("spider", "person", "angel", "bird", "demon")
val limits = 10..100 step 10
val startHours = listOf(8, 9, 10, null)

@Singleton
@Requires(property = "backend.fake.enabled", value = "true")
class FakeBackend : Backend {
    var namespaces = List(10) { fakeIt() }.map { it.name to it }.toMap()

    private fun fakeIt(): NamespaceStatus {
        val now = currentTimeMillis()
        val limit = limits.shuffled().first()
        val up = Random.Default.nextBoolean()
        val used = if (up) 0.rangeTo(limit).shuffled().first() else 0
        val stopAt = if (up) now + 0.rangeTo(8).shuffled().first() * 60 * 60 * 1000 else now - 1000
        return NamespaceStatus(
                "${adjectives.shuffled().first()}-${nouns.shuffled().first()}",
                up, used, limit,
                startHours.shuffled().first(),
                stopAt)
    }

    @Synchronized
    override fun getStatus(): Status {
        return Status(namespaces.values.toList().sortedBy { it.name })
    }

    @Synchronized
    override fun setMemLimit(namespace: String, limit: Int): Status {
        val ns = namespaces[namespace] ?: throw Exception("Namespace $namespace not found")
        update(ns.copy(memLimit = limit))
        return getStatus()
    }

    @Synchronized
    override fun setStartHour(namespace: String, startHour: Int?): Status {
        val ns = namespaces[namespace] ?: throw Exception("Namespace $namespace not found")
        update(ns.copy(startHour = startHour))
        return getStatus()
    }

    @Synchronized
    override fun extend(namespace: String): Status {
        val ns = namespaces[namespace] ?: throw Exception("Namespace $namespace not found")
        update(ns.copy(stopTime = currentTimeMillis() + 8 * 60 * 60 * 1000,
                up = true,
                memUsed = 0.rangeTo(ns.memLimit).shuffled().first()))
        return getStatus()
    }

    private fun update(ns: NamespaceStatus): Status {
        namespaces = namespaces.plus(ns.name to ns)
        return Status(namespaces.values.toList())
    }
}
