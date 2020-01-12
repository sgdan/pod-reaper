package org.sgdan.podreaper

import io.micronaut.http.annotation.Body
import io.micronaut.http.annotation.Controller
import io.micronaut.http.annotation.Get
import io.micronaut.http.annotation.Post
import kotlinx.coroutines.runBlocking

data class StartRequest(val namespace: String,
                        val startHour: Int?)

data class LimitRequest(val namespace: String,
                        val limit: Int)

@Controller("/")
class ReaperController(private val backend: Backend) {

    @Get("/reaper/status")
    fun status(): Status = runBlocking { backend.getStatus() }

    @Post("/reaper/setMemLimit")
    fun setMemLimit(@Body req: LimitRequest) = runBlocking {
        backend.setMemLimit(req.namespace, req.limit)
    }

    @Post("/reaper/setStartHour")
    fun setStartHour(@Body req: StartRequest) = runBlocking {
        backend.setStartHour(req.namespace, req.startHour)
    }

    @Post("/reaper/extend")
    fun postExtend(@Body req: StartRequest) = runBlocking {
        backend.extend(req.namespace)
    }
}
